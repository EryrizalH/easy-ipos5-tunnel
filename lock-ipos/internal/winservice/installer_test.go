package winservice

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveBundlePaths_Success(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, "nssm.exe"))
	mustWrite(t, filepath.Join(tmp, "pgbouncer.exe"))
	mustWrite(t, filepath.Join(tmp, "libevent-7.dll"))
	mustWrite(t, filepath.Join(tmp, "libssl-3-x64.dll"))
	mustWrite(t, filepath.Join(tmp, "libcrypto-3-x64.dll"))
	mustWrite(t, filepath.Join(tmp, "libwinpthread-1.dll"))
	mustWrite(t, filepath.Join(tmp, "client.toml"))
	mustWrite(t, filepath.Join(tmp, "ipos5-rathole.exe"))
	mustWrite(t, filepath.Join(tmp, guiBinaryName))

	paths, err := ResolveBundlePaths(tmp)
	if err != nil {
		t.Fatalf("ResolveBundlePaths() error = %v", err)
	}

	if !strings.HasSuffix(strings.ToLower(paths.NSSMPath), "nssm.exe") {
		t.Fatalf("NSSMPath unexpected: %s", paths.NSSMPath)
	}
	if !strings.HasSuffix(strings.ToLower(paths.RatholePath), "ipos5-rathole.exe") {
		t.Fatalf("RatholePath unexpected: %s", paths.RatholePath)
	}
	if !strings.HasSuffix(strings.ToLower(paths.ClientTomlPath), "client.toml") {
		t.Fatalf("ClientTomlPath unexpected: %s", paths.ClientTomlPath)
	}
	if !strings.HasSuffix(strings.ToLower(paths.GUIPath), guiBinaryName) {
		t.Fatalf("GUIPath unexpected: %s", paths.GUIPath)
	}
	if !strings.HasSuffix(strings.ToLower(paths.PgBouncerPath), "pgbouncer.exe") {
		t.Fatalf("PgBouncerPath unexpected: %s", paths.PgBouncerPath)
	}
	if !strings.HasSuffix(strings.ToLower(paths.PgBouncerDBsPath), strings.ToLower(pgBouncerDBsName)) {
		t.Fatalf("PgBouncerDBsPath unexpected: %s", paths.PgBouncerDBsPath)
	}
}

func TestResolveBundlePaths_MissingFiles(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, "nssm.exe"))

	_, err := ResolveBundlePaths(tmp)
	if err == nil {
		t.Fatal("expected error when required sidecar files are missing")
	}

	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "client.toml") {
		t.Fatalf("error should mention client.toml, got: %v", err)
	}
	if !strings.Contains(msg, "ipos5-rathole.exe/rathole.exe") {
		t.Fatalf("error should mention rathole binary, got: %v", err)
	}
	if !strings.Contains(msg, guiBinaryName) {
		t.Fatalf("error should mention GUI binary, got: %v", err)
	}
	if !strings.Contains(msg, "pgbouncer.exe") {
		t.Fatalf("error should mention pgbouncer.exe, got: %v", err)
	}
	if !strings.Contains(msg, "libevent-7.dll") {
		t.Fatalf("error should mention libevent-7.dll, got: %v", err)
	}
	if !strings.Contains(msg, "libwinpthread-1.dll") {
		t.Fatalf("error should mention libwinpthread-1.dll, got: %v", err)
	}
}

func TestBuildInstallCommands(t *testing.T) {
	cfg := Config{ServiceName: "EasyRatholeClient", BundleDir: `D:\bundle`}
	paths := BundlePaths{
		NSSMPath:          `D:\bundle\nssm.exe`,
		RatholePath:       `D:\bundle\ipos5-rathole.exe`,
		GUIPath:           `D:\bundle\ipos5-rathole-gui.exe`,
		ClientTomlPath:    `D:\bundle\client.toml`,
		PgBouncerPath:     `D:\bundle\pgbouncer.exe`,
		PgBouncerIniPath:  `D:\bundle\pgbouncer.ini`,
		PgBouncerUserPath: `D:\bundle\userlist.txt`,
	}

	cmds := BuildInstallCommands(cfg, paths, `C:\ProgramData\easy-rathole-client\logs`)
	if len(cmds) < 5 {
		t.Fatalf("expected install commands, got %d", len(cmds))
	}

	if got := strings.Join(cmds[0], " "); !strings.Contains(got, "install EasyRatholeClient") {
		t.Fatalf("unexpected first command: %s", got)
	}

	hasAutoStart := false
	for _, cmd := range cmds {
		if strings.Join(cmd, " ") == "set EasyRatholeClient Start SERVICE_AUTO_START" {
			hasAutoStart = true
			break
		}
	}
	if !hasAutoStart {
		t.Fatal("missing Start SERVICE_AUTO_START command")
	}
}

func TestBuildPgBouncerInstallCommands(t *testing.T) {
	paths := BundlePaths{
		NSSMPath:         `D:\bundle\nssm.exe`,
		PgBouncerPath:    `D:\bundle\pgbouncer.exe`,
		PgBouncerIniPath: `D:\bundle\pgbouncer.ini`,
	}

	cmds := BuildPgBouncerInstallCommands(paths, `C:\ProgramData\easy-rathole-client\logs`)
	if len(cmds) < 5 {
		t.Fatalf("expected pgbouncer install commands, got %d", len(cmds))
	}
	if got := strings.Join(cmds[0], " "); !strings.Contains(got, "install PgBouncer") {
		t.Fatalf("unexpected first command: %s", got)
	}
}

func TestBuildPgBouncerIni_DefaultFallback(t *testing.T) {
	got := buildPgBouncerIni(nil)
	if !strings.Contains(got, "postgres = host=127.0.0.1 port=5444 dbname=postgres") {
		t.Fatalf("expected default postgres mapping, got %s", got)
	}
	if strings.Contains(got, "* = host=127.0.0.1") {
		t.Fatalf("expected explicit database mapping instead of wildcard, got %s", got)
	}
}

func TestBuildPgBouncerIni_MultiDatabase(t *testing.T) {
	got := buildPgBouncerIni([]pgBouncerDatabaseEntry{{Name: "iposdb"}, {Name: "masterdb", BackendDBName: "master_backend"}})
	if !strings.Contains(got, "iposdb = host=127.0.0.1 port=5444 dbname=iposdb") {
		t.Fatalf("expected iposdb mapping, got %s", got)
	}
	if !strings.Contains(got, "masterdb = host=127.0.0.1 port=5444 dbname=master_backend") {
		t.Fatalf("expected aliased backend mapping, got %s", got)
	}
}

func TestLoadPgBouncerDatabaseEntries_DefaultWhenMissing(t *testing.T) {
	entries, err := loadPgBouncerDatabaseEntries(filepath.Join(t.TempDir(), pgBouncerDBsName))
	if err != nil {
		t.Fatalf("loadPgBouncerDatabaseEntries() unexpected error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "postgres" || entries[0].BackendDBName != "postgres" {
		t.Fatalf("unexpected default entries: %#v", entries)
	}
}

func TestLoadPgBouncerDatabaseEntries_FromJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, pgBouncerDBsName)
	payload, err := json.Marshal(pgBouncerDatabasesFile{Databases: []pgBouncerDatabaseEntry{{Name: "iposdb"}, {Name: " masterdb ", BackendDBName: " backenddb "}, {Name: "iposdb"}, {Name: ""}}})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	entries, err := loadPgBouncerDatabaseEntries(path)
	if err != nil {
		t.Fatalf("loadPgBouncerDatabaseEntries() unexpected error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 normalized entries, got %#v", entries)
	}
	if entries[0].Name != "iposdb" || entries[0].BackendDBName != "iposdb" {
		t.Fatalf("unexpected first entry: %#v", entries[0])
	}
	if entries[1].Name != "masterdb" || entries[1].BackendDBName != "backenddb" {
		t.Fatalf("unexpected second entry: %#v", entries[1])
	}
}

func TestLoadPgBouncerDatabaseEntries_EmptyJSONFails(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, pgBouncerDBsName)
	if err := os.WriteFile(path, []byte(`{"databases":[]}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := loadPgBouncerDatabaseEntries(path)
	if err == nil {
		t.Fatal("expected error for empty database config")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "kosong") {
		t.Fatalf("expected empty-config error, got %v", err)
	}
}

func TestBuildPgBouncerUserlist(t *testing.T) {
	got := buildPgBouncerUserlist()
	if !strings.Contains(got, "\"sysi5adm\"") {
		t.Fatalf("expected user in userlist, got %s", got)
	}
	if !strings.Contains(got, "md5") {
		t.Fatalf("expected md5 hash prefix, got %s", got)
	}
}

func TestBuildGUIShortcutSpec(t *testing.T) {
	spec := BuildGUIShortcutSpec(`D:\bundle`, `D:\bundle\ipos5-rathole-gui.exe`)
	if !strings.Contains(spec.LauncherPath, launcherFileName) {
		t.Fatalf("launcher path mismatch: %s", spec.LauncherPath)
	}
	if spec.ShortcutName != shortcutFileName {
		t.Fatalf("shortcut name mismatch: %s", spec.ShortcutName)
	}
	if !strings.Contains(strings.ToLower(spec.PowerShellArgs), "executionpolicy bypass") {
		t.Fatalf("powershell args mismatch: %s", spec.PowerShellArgs)
	}
	if strings.Contains(strings.ToLower(spec.PowerShellArgs), "--hidden") {
		t.Fatalf("powershell args should not force hidden mode: %s", spec.PowerShellArgs)
	}
}

func TestBuildLauncherContent_NoHiddenModeAndUsesRunAs(t *testing.T) {
	content := buildLauncherContent(`D:\bundle\ipos5-rathole-gui.exe`)
	lower := strings.ToLower(content)
	if strings.Contains(lower, "--hidden") {
		t.Fatalf("launcher should not force hidden mode: %s", content)
	}
	if !strings.Contains(lower, "-verb runas") {
		t.Fatalf("launcher must use RunAs: %s", content)
	}
	if !strings.Contains(content, "ipos5-rathole-gui.exe") {
		t.Fatalf("launcher must include GUI path: %s", content)
	}
}

func TestVerifyGUIArtifacts_Success(t *testing.T) {
	tmp := t.TempDir()
	launcher := filepath.Join(tmp, launcherFileName)
	shortcut := filepath.Join(tmp, shortcutFileName)
	mustWrite(t, launcher)
	mustWrite(t, shortcut)

	spec := GUIShortcutSpec{LauncherPath: launcher, ShortcutName: shortcutFileName}
	if err := verifyGUIArtifacts(spec, []string{shortcut}); err != nil {
		t.Fatalf("verifyGUIArtifacts() unexpected error: %v", err)
	}
}

func TestVerifyGUIArtifacts_FailsWithoutShortcut(t *testing.T) {
	tmp := t.TempDir()
	launcher := filepath.Join(tmp, launcherFileName)
	mustWrite(t, launcher)
	spec := GUIShortcutSpec{LauncherPath: launcher, ShortcutName: shortcutFileName}

	err := verifyGUIArtifacts(spec, nil)
	if err == nil {
		t.Fatal("expected error when shortcut list is empty")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "shortcut") {
		t.Fatalf("expected shortcut-related error, got: %v", err)
	}
}

func TestDesktopShortcutDirs(t *testing.T) {
	t.Setenv("USERPROFILE", `C:\Users\tester`)
	t.Setenv("PUBLIC", `C:\Users\Public`)
	dirs := desktopShortcutDirs()
	if len(dirs) != 2 {
		t.Fatalf("expected 2 desktop dirs, got %d", len(dirs))
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := osWrite(path); err != nil {
		t.Fatalf("failed creating file %s: %v", path, err)
	}
}

func osWrite(path string) error {
	return os.WriteFile(path, []byte("x"), 0o644)
}
