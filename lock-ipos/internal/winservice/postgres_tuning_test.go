package winservice

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lock-ipos/lock-ipos/internal/progress"
)

func effectiveTuning(ram uint64) map[string]string {
	out := map[string]string{}
	for _, s := range postgresTuningSettings(ram) {
		out[s.name] = s.effective
	}
	return out
}

func TestPostgresTuningMemory(t *testing.T) {
	for _, tc := range []struct{ ram, mb uint64 }{
		{256 << 20, 128}, {512 << 20, 128}, {1027 << 20, 256}, {2 << 30, 512}, {8 << 30, 512}, {^uint64(0), 512},
	} {
		got := postgresTuningSettings(tc.ram)
		if got[4].effective != strconv.FormatUint(tc.mb*128, 10) {
			t.Fatalf("RAM %d: unexpected shared_buffers %v", tc.ram, got[4])
		}
		if got[2].value != "10" || got[3].value != "0" || got[5].value != "8MB" {
			t.Fatalf("unexpected Windows tuning: %v", got)
		}
	}
}

func TestRewritePostgresTuningPreservesEncodingAndIsIdempotent(t *testing.T) {
	original := "# operator\r\nport = 5445\r\ninclude = 'custom.conf'\r\n# shared_buffers = 256MB\r\n"
	for name, raw := range map[string][]byte{
		"utf8":     []byte(original),
		"utf8-bom": append([]byte{0xef, 0xbb, 0xbf}, []byte(original)...),
		"legacy":   append([]byte("# legacy \xe9\r\n"), []byte(original)...),
		"utf16-le": {0xff, 0xfe}, "utf16-be": {0xfe, 0xff},
	} {
		t.Run(name, func(t *testing.T) {
			if strings.HasPrefix(name, "utf16") {
				text, _ := decodePostgresConfig(raw)
				raw = text.encode(original)
			}
			settings := postgresTuningSettings(4 << 30)
			updated, err := rewritePostgresTuning(raw, settings, false)
			if err != nil || !bytes.HasPrefix(updated, raw) {
				t.Fatalf("original config/encoding changed: %v", err)
			}
			again, err := rewritePostgresTuning(updated, settings, false)
			if err != nil || !bytes.Equal(updated, again) {
				t.Fatalf("repeated install changes config: %v", err)
			}
			decoded, _ := decodePostgresConfig(updated)
			if strings.Count(decoded.text, tuningBlockStart) != 1 || strings.Contains(strings.ReplaceAll(decoded.text, "\r\n", ""), "\n") {
				t.Fatal("duplicate block or CRLF lost")
			}
		})
	}
}

func TestRewritePostgresAutoConfRemovesOnlyTuningOverrides(t *testing.T) {
	raw := []byte("# PostgreSQL auto config\r\nport = '5445'\r\n# max_connections = 999\r\nmax_connections = '1000' # operator note\r\nlog_line_prefix = '#not a comment' # logging note\r\narchive_command = 'copy %p X:\\backup'\r\n")
	updated, err := rewritePostgresTuning(raw, postgresTuningSettings(4<<30), true)
	if err != nil {
		t.Fatal(err)
	}
	want := "# PostgreSQL auto config\r\nport = '5445'\r\n# max_connections = 999\r\n# operator note\r\n# logging note\r\narchive_command = 'copy %p X:\\backup'\r\n"
	if string(updated) != want {
		t.Fatalf("unrelated config/comments changed: %q", updated)
	}
	for _, bad := range []string{tuningBlockStart, tuningBlockEnd, tuningBlockStart + "\n" + tuningBlockStart} {
		if _, err := rewritePostgresTuning([]byte(bad), postgresTuningSettings(4<<30), false); err == nil {
			t.Fatalf("malformed block accepted: %s", bad)
		}
	}
}

type tuningFixture struct {
	target                 postgresTarget
	hooks                  postgresTuningHooks
	before, expected       map[string]string
	original, autoOriginal []byte
	restarts, verifies     int
	backups, writes        int
}

func newTuningFixture(t *testing.T) *tuningFixture {
	t.Helper()
	dir := t.TempDir()
	f := &tuningFixture{
		target:       postgresTarget{filepath.Join(dir, "postgresql.conf"), dir, "selected-postgres", 5445},
		original:     []byte("# operator config\r\nport = 5445\r\n"),
		autoOriginal: []byte("port = '5445'\r\nmax_connections = '1000'\r\n"),
		expected:     effectiveTuning(4 << 30), before: effectiveTuning(1 << 30),
	}
	f.before["max_connections"] = "1000"
	for path, data := range map[string][]byte{f.target.ConfigPath: f.original, filepath.Join(dir, "postgresql.auto.conf"): f.autoOriginal} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	f.hooks = postgresTuningHooks{
		detect:   func(string) (postgresTarget, error) { return f.target, nil },
		memory:   func() (uint64, error) { return 4 << 30, nil },
		settings: func(string, int) (map[string]string, error) { return f.before, nil },
		backup:   func(file postgresTuningFile) (string, error) { f.backups++; return backupPostgresTuningFile(file) },
		write: func(path string, data []byte, mode os.FileMode) error {
			f.writes++
			return os.WriteFile(path, data, mode)
		},
		restart: func(target postgresTarget) error {
			if target != f.target {
				t.Fatal("wrong instance restarted")
			}
			f.restarts++
			return nil
		},
		verify: func(_ string, port int, _ map[string]string) error {
			if port != 5445 {
				t.Fatal("port changed")
			}
			f.verifies++
			return nil
		},
	}
	return f
}

func (f *tuningFixture) assertRestored(t *testing.T) {
	t.Helper()
	for path, want := range map[string][]byte{f.target.ConfigPath: f.original, filepath.Join(f.target.DataDir, "postgresql.auto.conf"): f.autoOriginal} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("config not restored at %s: %v", path, err)
		}
	}
}

func TestPostgresTuningWorkflowAndRepeatedInstall(t *testing.T) {
	f := newTuningFixture(t)
	r := &captureReporter{}
	if err := tunePostgresWithHooks("selected-bin", r, f.hooks); err != nil {
		t.Fatal(err)
	}
	if f.backups != 2 || f.writes != 2 || f.restarts != 1 || f.verifies != 1 {
		t.Fatalf("unexpected calls: %+v", f)
	}
	backupFiles, err := filepath.Glob(filepath.Join(f.target.DataDir, "*.nusatunnel-backup-*"))
	if err != nil || len(backupFiles) != 2 {
		t.Fatalf("missing backups: %v %v", backupFiles, err)
	}
	for _, path := range backupFiles {
		data, _ := os.ReadFile(path)
		if !bytes.Equal(data, f.original) && !bytes.Equal(data, f.autoOriginal) {
			t.Fatal("wrong backup bytes")
		}
	}
	f.before = f.expected
	if err := tunePostgresWithHooks("selected-bin", r, f.hooks); err != nil {
		t.Fatal(err)
	}
	if f.backups != 2 || f.writes != 2 || f.restarts != 1 || f.verifies != 2 {
		t.Fatal("repeat install should not rewrite/restart")
	}
	var ids []string
	for _, event := range r.steps[:10] {
		if strings.HasPrefix(event, "start:") {
			ids = append(ids, strings.Split(event, ":")[1])
		}
	}
	wantIDs := []string{"tune-postgres-detect", "tune-postgres-backup", "tune-postgres-apply", "tune-postgres-restart", "tune-postgres-verify"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("wrong step order: %v", ids)
	}
}

func TestPostgresTuningFailuresRestoreOriginals(t *testing.T) {
	for _, stage := range []string{"detect", "memory", "zero-memory", "read", "settings", "backup", "partial-write", "second-write", "restart", "verify", "rollback-restart"} {
		t.Run(stage, func(t *testing.T) {
			f := newTuningFixture(t)
			fail := errors.New("injected " + stage)
			switch stage {
			case "detect":
				f.hooks.detect = func(string) (postgresTarget, error) { return postgresTarget{}, fail }
			case "memory":
				f.hooks.memory = func() (uint64, error) { return 0, fail }
			case "zero-memory":
				f.hooks.memory = func() (uint64, error) { return 0, nil }
			case "read":
				f.hooks.detect = func(string) (postgresTarget, error) {
					target := f.target
					target.ConfigPath += ".missing"
					return target, nil
				}
			case "settings":
				f.hooks.settings = func(string, int) (map[string]string, error) { return nil, fail }
			case "backup":
				f.hooks.backup = func(file postgresTuningFile) (string, error) {
					f.backups++
					if f.backups == 2 {
						return "", fail
					}
					return backupPostgresTuningFile(file)
				}
			case "partial-write", "second-write":
				f.hooks.write = func(path string, data []byte, mode os.FileMode) error {
					f.writes++
					if (stage == "partial-write" && f.writes == 1) || (stage == "second-write" && f.writes == 2) {
						_ = os.WriteFile(path, []byte("partial"), mode)
						return fail
					}
					return os.WriteFile(path, data, mode)
				}
			case "restart", "rollback-restart":
				f.hooks.restart = func(postgresTarget) error {
					f.restarts++
					if f.restarts == 1 || stage == "rollback-restart" {
						return fail
					}
					return nil
				}
			case "verify":
				f.hooks.verify = func(_ string, _ int, expected map[string]string) error {
					f.verifies++
					if f.verifies == 1 {
						return fail
					}
					if !reflect.DeepEqual(expected, f.before) {
						t.Fatal("rollback must verify old settings")
					}
					return nil
				}
			}
			err := tunePostgresWithHooks("selected-bin", progress.NopReporter(), f.hooks)
			if err == nil {
				t.Fatal("injected failure ignored")
			}
			f.assertRestored(t)
			if stage == "backup" && f.writes != 0 {
				t.Fatal("writes began before all backups succeeded")
			}
			if stage == "rollback-restart" && !strings.Contains(err.Error(), "rollback gagal") {
				t.Fatal("recovery failure hidden")
			}
			if (stage == "partial-write" || stage == "second-write") && f.restarts != 0 {
				t.Fatal("write failure should not restart an unchanged running DB")
			}
		})
	}
}

func TestTuningRollbackWriteFailureIsVisible(t *testing.T) {
	f := newTuningFixture(t)
	f.hooks.write = func(string, []byte, os.FileMode) error { return errors.New("disk unavailable") }
	err := tunePostgresWithHooks("selected-bin", progress.NopReporter(), f.hooks)
	if err == nil || !strings.Contains(err.Error(), "rollback gagal") || f.restarts != 0 {
		t.Fatalf("recovery failure hidden: %v", err)
	}
}

func TestTuningDoesNotRestartWhenEffectiveValuesAlreadyMatch(t *testing.T) {
	f := newTuningFixture(t)
	f.before = f.expected
	if err := tunePostgresWithHooks("selected-bin", progress.NopReporter(), f.hooks); err != nil {
		t.Fatal(err)
	}
	if f.restarts != 0 || f.verifies != 1 || f.writes != 2 {
		t.Fatal("only persist config when effective values already match")
	}
}

func TestTuningRestartsWhenDiskAlreadyTunedButEffectiveSettingsDiffer(t *testing.T) {
	f := newTuningFixture(t)
	settings := postgresTuningSettings(4 << 30)
	config, _ := rewritePostgresTuning(f.original, settings, false)
	auto, _ := rewritePostgresTuning(f.autoOriginal, settings, true)
	_ = os.WriteFile(f.target.ConfigPath, config, 0600)
	_ = os.WriteFile(filepath.Join(f.target.DataDir, "postgresql.auto.conf"), auto, 0600)
	if err := tunePostgresWithHooks("selected-bin", progress.NopReporter(), f.hooks); err != nil {
		t.Fatal(err)
	}
	if f.writes != 0 || f.backups != 0 || f.restarts != 1 {
		t.Fatal("pending effective changes require restart without rewriting files")
	}
}

func TestTuningDoesNotCreateMissingAutoConf(t *testing.T) {
	f := newTuningFixture(t)
	path := filepath.Join(f.target.DataDir, "postgresql.auto.conf")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := tunePostgresWithHooks("selected-bin", progress.NopReporter(), f.hooks); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected auto config: %v", err)
	}
	if f.backups != 1 || f.writes != 1 {
		t.Fatal("only main config should be backed up and changed")
	}
}

func TestTuningPreservesConcurrentAutoConfChange(t *testing.T) {
	f := newTuningFixture(t)
	path := filepath.Join(f.target.DataDir, "postgresql.auto.conf")
	changed := []byte("port = '5445'\r\noperator_setting = 'changed'\r\n")
	f.hooks.backup = func(file postgresTuningFile) (string, error) {
		f.backups++
		backup, err := backupPostgresTuningFile(file)
		if f.backups == 2 {
			_ = os.WriteFile(path, changed, 0600)
		}
		return backup, err
	}
	err := tunePostgresWithHooks("selected-bin", progress.NopReporter(), f.hooks)
	if err == nil || !strings.Contains(err.Error(), "berubah saat tuning") {
		t.Fatalf("concurrent change ignored: %v", err)
	}
	got, _ := os.ReadFile(path)
	config, _ := os.ReadFile(f.target.ConfigPath)
	if !bytes.Equal(got, changed) || !bytes.Equal(config, f.original) || f.restarts != 0 {
		t.Fatal("rollback must preserve concurrent operator changes")
	}
}

func TestIPPublicTuningRunsBeforeTunnelAndFailureAbortsTunnel(t *testing.T) {
	for _, fail := range []bool{false, true} {
		var calls []string
		h := installModeHandlers{
			tunePostgres: func(string, progress.Reporter) error {
				calls = append(calls, "tuning")
				if fail {
					return errors.New("tuning failed")
				}
				return nil
			},
			installTunnelService: func(Config, BundlePaths, string, progress.Reporter, commandRunner) error {
				calls = append(calls, "tunnel")
				return nil
			},
		}
		err := executeInstallModeWithHandlers(Config{InstallMode: InstallModeIPPublicOnly}, BundlePaths{}, "", progress.NopReporter(), commandRunner{}, h)
		want := []string{"tuning", "tunnel"}
		if fail {
			want = []string{"tuning"}
		}
		if (err != nil) != fail || !reflect.DeepEqual(calls, want) {
			t.Fatalf("wrong sequencing: %v %v", calls, err)
		}
		calls = nil
		h.migratePostgresPort = func(string) error { calls = append(calls, "migrate"); return nil }
		h.installPgBouncer = func(Config, BundlePaths, string, progress.Reporter, commandRunner) error {
			calls = append(calls, "pgbouncer")
			return nil
		}
		h.waitPgBouncerHealth = func(string, time.Duration, progress.Reporter) error { calls = append(calls, "health"); return nil }
		if err := executeInstallModeWithHandlers(Config{InstallMode: InstallModePgBouncerOnly}, BundlePaths{}, "", progress.NopReporter(), commandRunner{}, h); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(calls, []string{"migrate", "pgbouncer", "health"}) {
			t.Fatalf("PgBouncer must complete without tuning/tunnel: %v", calls)
		}
	}
}
