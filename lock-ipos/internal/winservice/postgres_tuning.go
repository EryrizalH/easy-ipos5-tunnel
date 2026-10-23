package winservice

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/lock-ipos/lock-ipos/internal/db"
	"github.com/lock-ipos/lock-ipos/internal/progress"
)

const tuningBlockStart = "# BEGIN NusaTunnel PostgreSQL tuning"
const tuningBlockEnd = "# END NusaTunnel PostgreSQL tuning"
const tuningLogPrefix = "%t [%p]: user=%u,db=%d,client=%h "

type postgresTarget struct {
	ConfigPath  string
	DataDir     string
	ServiceName string
	Port        int
}

type postgresTuningSetting struct{ name, value, effective string }

func postgresTuningSettings(ramBytes uint64) []postgresTuningSetting {
	mb := ramBytes / (1024 * 1024) / 4
	if mb < 128 {
		mb = 128
	}
	if mb > 512 {
		mb = 512
	}
	return []postgresTuningSetting{
		{"max_connections", "200", "200"},
		{"tcp_keepalives_idle", "60", "60"},
		{"tcp_keepalives_interval", "10", "10"},
		{"tcp_keepalives_count", "0", "0"},
		{"shared_buffers", fmt.Sprintf("%dMB", mb), strconv.FormatUint(mb*128, 10)}, // pg_settings: 8kB blocks
		{"work_mem", "8MB", "8192"},                                                 // pg_settings: kB
		{"log_connections", "on", "on"},
		{"log_disconnections", "on", "on"},
		{"log_line_prefix", "'" + tuningLogPrefix + "'", tuningLogPrefix},
	}
}

// Config bytes outside our block are kept verbatim, including legacy encodings.
// UTF-16 is decoded only with a BOM; inserted text is ASCII in all encodings.
type postgresConfigText struct {
	text string
	bom  []byte
	wide binary.ByteOrder
}

func decodePostgresConfig(raw []byte) (postgresConfigText, error) {
	t := postgresConfigText{}
	if bytes.HasPrefix(raw, []byte{0xff, 0xfe}) {
		t.bom, t.wide, raw = raw[:2], binary.LittleEndian, raw[2:]
	} else if bytes.HasPrefix(raw, []byte{0xfe, 0xff}) {
		t.bom, t.wide, raw = raw[:2], binary.BigEndian, raw[2:]
	} else if bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf}) {
		t.bom, raw = raw[:3], raw[3:]
	}
	if t.wide != nil {
		if len(raw)%2 != 0 {
			return t, errors.New("encoding UTF-16 konfigurasi PostgreSQL tidak valid")
		}
		units := make([]uint16, len(raw)/2)
		for i := range units {
			units[i] = t.wide.Uint16(raw[i*2:])
		}
		for i := 0; i < len(units); i++ {
			if units[i] >= 0xd800 && units[i] <= 0xdbff {
				if i+1 == len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
					return t, errors.New("surrogate UTF-16 konfigurasi tidak valid")
				}
				i++
			} else if units[i] >= 0xdc00 && units[i] <= 0xdfff {
				return t, errors.New("surrogate UTF-16 konfigurasi tidak valid")
			}
		}
		t.text = string(utf16.Decode(units))
	} else {
		t.text = string(raw)
	}
	return t, nil
}

func (t postgresConfigText) encode(s string) []byte {
	out := append([]byte(nil), t.bom...)
	if t.wide == nil {
		return append(out, []byte(s)...)
	}
	for _, unit := range utf16.Encode([]rune(s)) {
		var b [2]byte
		t.wide.PutUint16(b[:], unit)
		out = append(out, b[:]...)
	}
	return out
}

var postgresAssignment = regexp.MustCompile(`^\s*([a-zA-Z_][a-zA-Z_0-9]*)\s*=`)

func rewritePostgresTuning(raw []byte, settings []postgresTuningSetting, auto bool) ([]byte, error) {
	t, err := decodePostgresConfig(raw)
	if err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for _, setting := range settings {
		keys[setting.name] = true
	}
	newline := "\n"
	if strings.Contains(t.text, "\r\n") {
		newline = "\r\n"
	}
	var out strings.Builder
	inBlock := false
	for _, line := range strings.SplitAfter(t.text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !auto && trimmed == tuningBlockStart {
			if inBlock {
				return nil, errors.New("blok tuning PostgreSQL bersarang; periksa konfigurasi")
			}
			inBlock = true
			continue
		}
		if !auto && trimmed == tuningBlockEnd {
			if !inBlock {
				return nil, errors.New("penutup blok tuning PostgreSQL tanpa pembuka")
			}
			inBlock = false
			continue
		}
		if inBlock {
			continue
		}
		if auto {
			match := postgresAssignment.FindStringSubmatch(line)
			if len(match) == 2 && keys[strings.ToLower(match[1])] {
				// Retain an inline operator comment when removing the override.
				if comment := postgresInlineComment(line); comment != "" {
					out.WriteString(comment)
				}
				continue
			}
		}
		out.WriteString(line)
	}
	if inBlock {
		return nil, errors.New("blok tuning PostgreSQL belum ditutup")
	}
	if !auto {
		if out.Len() > 0 && !strings.HasSuffix(out.String(), "\n") {
			out.WriteString(newline)
		}
		out.WriteString(tuningBlockStart + newline)
		for _, setting := range settings {
			out.WriteString(setting.name + " = " + setting.value + newline)
		}
		out.WriteString(tuningBlockEnd + newline)
	}
	return t.encode(out.String()), nil
}

func postgresInlineComment(line string) string {
	quoted := false
	for i := 0; i < len(line); i++ {
		if quoted && line[i] == '\\' {
			i++
			continue
		}
		if line[i] == '\'' {
			if quoted && i+1 < len(line) && line[i+1] == '\'' {
				i++
				continue
			}
			quoted = !quoted
		}
		if !quoted && line[i] == '#' {
			return line[i:]
		}
	}
	return ""
}

type postgresTuningFile struct {
	path              string
	original, updated []byte
	mode              os.FileMode
}

type postgresTuningHooks struct {
	detect   func(string) (postgresTarget, error)
	memory   func() (uint64, error)
	settings func(string, int) (map[string]string, error)
	backup   func(postgresTuningFile) (string, error)
	write    func(string, []byte, os.FileMode) error
	restart  func(postgresTarget) error
	verify   func(string, int, map[string]string) error
}

func tunePostgresWithProgress(pgBinPath string, reporter progress.Reporter) error {
	return tunePostgresWithHooks(pgBinPath, reporter, postgresTuningHooks{
		detect: discoverPostgresTarget, memory: systemPhysicalMemory,
		settings: readPostgresTuningSettings, backup: backupPostgresTuningFile,
		write: os.WriteFile, restart: restartTunedPostgres, verify: verifyPostgresTuning,
	})
}

func tunePostgresWithHooks(pgBinPath string, reporter progress.Reporter, h postgresTuningHooks) (result error) {
	reporter = normalizeReporter(reporter)
	step := func(id, label string, action func() error) error {
		reporter.StartStep(id, label)
		err := action()
		if err != nil {
			reporter.FinishStep(id, false, err.Error())
		} else {
			reporter.FinishStep(id, true, "Selesai")
		}
		return err
	}
	var target postgresTarget
	var files []postgresTuningFile
	var before, expected map[string]string
	err := step("tune-postgres-detect", "Deteksi konfigurasi dan RAM PostgreSQL", func() error {
		var err error
		target, err = h.detect(pgBinPath)
		if err != nil {
			return err
		}
		ram, err := h.memory()
		if err != nil {
			return fmt.Errorf("deteksi RAM gagal: %w", err)
		}
		if ram == 0 {
			return errors.New("deteksi RAM gagal: hasil nol")
		}
		settings := postgresTuningSettings(ram)
		expected = make(map[string]string, len(settings))
		for _, s := range settings {
			expected[s.name] = s.effective
		}
		before, err = h.settings(pgBinPath, target.Port)
		if err != nil {
			return err
		}
		if len(before) != len(expected) {
			return errors.New("parameter PostgreSQL tidak lengkap; tuning dibatalkan")
		}
		for key := range expected {
			if _, ok := before[key]; !ok {
				return fmt.Errorf("parameter PostgreSQL %s tidak ditemukan", key)
			}
		}
		if filepath.Clean(target.ConfigPath) == filepath.Join(target.DataDir, "postgresql.auto.conf") {
			return errors.New("config_file tidak boleh sama dengan postgresql.auto.conf")
		}
		for i, path := range []string{target.ConfigPath, filepath.Join(target.DataDir, "postgresql.auto.conf")} {
			raw, err := os.ReadFile(path)
			if i == 1 && errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return fmt.Errorf("gagal membaca %s: %w", path, err)
			}
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			updated, err := rewritePostgresTuning(raw, settings, i == 1)
			if err != nil {
				return err
			}
			if !bytes.Equal(raw, updated) {
				files = append(files, postgresTuningFile{path, raw, updated, info.Mode().Perm()})
			}
		}
		reporter.Log(fmt.Sprintf("Tuning %s: shared_buffers=%s, port PostgreSQL tetap %d", target.ConfigPath, settings[4].value, target.Port))
		return nil
	})
	if err != nil {
		return err
	}
	if err := step("tune-postgres-backup", "Backup konfigurasi PostgreSQL", func() error {
		for _, file := range files {
			if err := tuningFileUnchanged(file); err != nil {
				return err
			}
			path, err := h.backup(file)
			if err != nil {
				return fmt.Errorf("backup %s gagal: %w", file.path, err)
			}
			reporter.Log("Backup konfigurasi: " + path)
		}
		return nil
	}); err != nil {
		return err
	}
	var attempted []postgresTuningFile
	restartAttempted := false
	defer func() {
		if result == nil || (len(attempted) == 0 && !restartAttempted) {
			return
		}
		reporter.Log("Tuning gagal; memulihkan konfigurasi PostgreSQL sebelumnya")
		var recoveryErrors []error
		for _, file := range attempted {
			if err := h.write(file.path, file.original, file.mode); err != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf("restore %s: %w", file.path, err))
			}
		}
		if len(recoveryErrors) == 0 && restartAttempted {
			if err := h.restart(target); err != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf("restart rollback: %w", err))
			} else if err := h.verify(pgBinPath, target.Port, before); err != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf("verifikasi rollback: %w", err))
			}
		}
		if len(recoveryErrors) > 0 {
			result = fmt.Errorf("%w; rollback gagal, gunakan backup konfigurasi: %v", result, errors.Join(recoveryErrors...))
		} else {
			result = fmt.Errorf("%w; konfigurasi PostgreSQL sebelumnya dipulihkan", result)
		}
		reporter.Log(result.Error())
	}()
	if err := step("tune-postgres-apply", "Menerapkan tuning PostgreSQL", func() error {
		for _, file := range files {
			if err := tuningFileUnchanged(file); err != nil {
				return err
			}
			attempted = append(attempted, file) // A failed write can already have truncated the file.
			if err := h.write(file.path, file.updated, file.mode); err != nil {
				return fmt.Errorf("gagal menulis %s: %w", file.path, err)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if err := step("tune-postgres-restart", "Restart PostgreSQL bila diperlukan", func() error {
		if tuningSettingsMatch(before, expected) {
			reporter.Log("Nilai efektif sudah sesuai; restart PostgreSQL dilewati")
			return nil
		}
		restartAttempted = true
		return h.restart(target)
	}); err != nil {
		return err
	}
	return step("tune-postgres-verify", "Verifikasi nilai efektif PostgreSQL", func() error {
		return h.verify(pgBinPath, target.Port, expected)
	})
}

func tuningSettingsMatch(actual, expected map[string]string) bool {
	for key, value := range expected {
		if actual[key] != value {
			return false
		}
	}
	return true
}

func tuningFileUnchanged(file postgresTuningFile) error {
	current, err := os.ReadFile(file.path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, file.original) {
		return fmt.Errorf("konfigurasi %s berubah saat tuning; proses dibatalkan", file.path)
	}
	return nil
}

func backupPostgresTuningFile(file postgresTuningFile) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(file.path), filepath.Base(file.path)+".nusatunnel-backup-*")
	if err != nil {
		return "", err
	}
	path := f.Name()
	_, writeErr := f.Write(file.original)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return "", err
	}
	return path, nil
}

func queryPostgresTuning(pgBinPath string, port int, sql string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(pgBinPath, "psql.exe"), "-X", "-A", "-t", "-w", "-v", "ON_ERROR_STOP=1",
		"-h", "127.0.0.1", "-p", strconv.Itoa(port), "-U", db.User, "-d", "postgres", "-c", sql)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+db.Password, "PGCONNECT_TIMEOUT=3", "PGCLIENTENCODING=UTF8")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		// Do not echo query output or connection credentials into installer logs.
		return "", fmt.Errorf("query tuning PostgreSQL port %d gagal: %w", port, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func readPostgresTuningSettings(pgBinPath string, port int) (map[string]string, error) {
	var names []string
	for _, s := range postgresTuningSettings(0) {
		names = append(names, "'"+s.name+"'")
	}
	out, err := queryPostgresTuning(pgBinPath, port, "SELECT json_object_agg(name, setting) FROM pg_settings WHERE name IN ("+strings.Join(names, ",")+");")
	if err != nil {
		return nil, err
	}
	var settings map[string]string
	if err := json.Unmarshal([]byte(out), &settings); err != nil {
		return nil, fmt.Errorf("hasil parameter PostgreSQL tidak valid: %w", err)
	}
	return settings, nil
}

func verifyPostgresTuning(pgBinPath string, port int, expected map[string]string) error {
	deadline := time.Now().Add(20 * time.Second)
	for {
		actual, err := readPostgresTuningSettings(pgBinPath, port)
		if err == nil {
			for key, value := range expected {
				if actual[key] != value {
					return fmt.Errorf("nilai efektif %s belum sesuai setelah tuning", key)
				}
			}
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("PostgreSQL belum siap setelah restart: %w", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
