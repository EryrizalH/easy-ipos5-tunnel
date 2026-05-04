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

func TestBuildPgBouncerFirewallCommands(t *testing.T) {
	deleteCmd := strings.Join(BuildPgBouncerFirewallDeleteCommand(), " ")
	if !strings.Contains(deleteCmd, "delete rule") {
		t.Fatalf("expected delete firewall command, got %s", deleteCmd)
	}
	if !strings.Contains(deleteCmd, pgBouncerFirewallRuleName) {
		t.Fatalf("expected firewall rule name in delete command, got %s", deleteCmd)
	}

	addCmd := strings.Join(BuildPgBouncerFirewallAddCommand(), " ")
	checks := []string{
		"add rule",
		pgBouncerFirewallRuleName,
		"localport=5444",
		"remoteip=any",
		"profile=any",
	}
	for _, needle := range checks {
		if !strings.Contains(addCmd, needle) {
			t.Fatalf("expected add firewall command to contain %q, got %s", needle, addCmd)
		}
	}
}

func TestBuildPgBouncerIni_DefaultFallback(t *testing.T) {
	got := buildPgBouncerIni(nil)
	if !strings.Contains(got, "postgres = host=127.0.0.1 port=5445 dbname=postgres") {
		t.Fatalf("expected default postgres mapping, got %s", got)
	}
	if !strings.Contains(got, "listen_addr = 0.0.0.0") {
		t.Fatalf("expected listen_addr 0.0.0.0, got %s", got)
	}
	if !strings.Contains(got, "listen_port = 5444") {
		t.Fatalf("expected listen_port 5444, got %s", got)
	}
	if strings.Contains(got, "* = host=127.0.0.1") {
		t.Fatalf("expected explicit database mapping instead of wildcard, got %s", got)
	}
}

func TestBuildPgBouncerIni_MultiDatabase(t *testing.T) {
	got := buildPgBouncerIni([]pgBouncerDatabaseEntry{{Name: "iposdb"}, {Name: "masterdb", BackendDBName: "master_backend"}})
	if !strings.Contains(got, "iposdb = host=127.0.0.1 port=5445 dbname=iposdb") {
		t.Fatalf("expected iposdb mapping, got %s", got)
	}
	if !strings.Contains(got, "masterdb = host=127.0.0.1 port=5445 dbname=master_backend") {
		t.Fatalf("expected aliased backend mapping, got %s", got)
	}
}

func TestResolveBundlePaths_IPPublicModeAllowsMissingPgBouncer(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, "nssm.exe"))
	mustWrite(t, filepath.Join(tmp, "client.toml"))
	mustWrite(t, filepath.Join(tmp, "ipos5-rathole.exe"))
	mustWrite(t, filepath.Join(tmp, guiBinaryName))

	_, err := resolveBundlePaths(tmp, false)
	if err != nil {
		t.Fatalf("resolveBundlePaths(mode ip public) should allow missing PgBouncer assets, got %v", err)
	}
}

func TestEnsureDBForwardAddress_UpdatesLegacy6432(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "client.toml")
	if err := os.WriteFile(path, []byte("local_addr = \"127.0.0.1:6432\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := ensureDBForwardAddress(path); err != nil {
		t.Fatalf("ensureDBForwardAddress() error = %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(raw), "127.0.0.1:5444") {
		t.Fatalf("expected DB local_addr rewritten to 5444, got %s", string(raw))
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

func TestParsePostgresDatabaseEntriesOutput(t *testing.T) {
	out := " i5_asdas \r\n\r\ni5_hjghjg\n i5_asdas \n"
	entries := parsePostgresDatabaseEntriesOutput(out)
	if len(entries) != 2 {
		t.Fatalf("expected 2 normalized entries, got %#v", entries)
	}
	if entries[0].Name != "i5_asdas" || entries[0].BackendDBName != "i5_asdas" {
		t.Fatalf("unexpected first entry: %#v", entries[0])
	}
	if entries[1].Name != "i5_hjghjg" || entries[1].BackendDBName != "i5_hjghjg" {
		t.Fatalf("unexpected second entry: %#v", entries[1])
	}
}

func TestBuildPgBouncerDatabasesJSON(t *testing.T) {
	raw, err := buildPgBouncerDatabasesJSON([]pgBouncerDatabaseEntry{{Name: " i5_asdas "}, {Name: "i5_hjghjg", BackendDBName: " i5_hjghjg "}, {Name: "i5_asdas"}})
	if err != nil {
		t.Fatalf("buildPgBouncerDatabasesJSON() unexpected error = %v", err)
	}

	var cfg pgBouncerDatabasesFile
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(cfg.Databases) != 2 {
		t.Fatalf("expected 2 normalized database entries, got %#v", cfg.Databases)
	}
	if cfg.Databases[0].Name != "i5_asdas" || cfg.Databases[0].BackendDBName != "i5_asdas" {
		t.Fatalf("unexpected first entry: %#v", cfg.Databases[0])
	}
	if cfg.Databases[1].Name != "i5_hjghjg" || cfg.Databases[1].BackendDBName != "i5_hjghjg" {
		t.Fatalf("unexpected second entry: %#v", cfg.Databases[1])
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

func TestSyncPgBouncerFirewallRuleWithProgress_IsIdempotentAcrossReruns(t *testing.T) {
	t.Helper()
	var calls []string
	ruleExists := false

	runFn := func(name string, args ...string) (string, error) {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		if name != "netsh" {
			t.Fatalf("expected netsh command, got %s", name)
		}
		if len(args) < 4 {
			t.Fatalf("unexpected args: %#v", args)
		}
		switch args[2] {
		case "show":
			if ruleExists {
				return "Rule Name: " + pgBouncerFirewallRuleName, nil
			}
			return "No rules match the specified criteria.", nil
		case "delete":
			ruleExists = false
			return "Deleted 1 rule(s).", nil
		case "add":
			ruleExists = true
			return "Ok.", nil
		default:
			t.Fatalf("unexpected firewall action: %#v", args)
			return "", nil
		}
	}

	if err := syncPgBouncerFirewallRuleWithProgress(nil, runFn); err != nil {
		t.Fatalf("first sync unexpected error = %v", err)
	}
	if err := syncPgBouncerFirewallRuleWithProgress(nil, runFn); err != nil {
		t.Fatalf("second sync unexpected error = %v", err)
	}
	if !ruleExists {
		t.Fatal("expected firewall rule to exist after rerun")
	}

	expected := []string{
		"netsh advfirewall firewall show rule name=" + pgBouncerFirewallRuleName,
		"netsh " + strings.Join(BuildPgBouncerFirewallAddCommand(), " "),
		"netsh advfirewall firewall show rule name=" + pgBouncerFirewallRuleName,
		"netsh " + strings.Join(BuildPgBouncerFirewallDeleteCommand(), " "),
		"netsh " + strings.Join(BuildPgBouncerFirewallAddCommand(), " "),
	}
	if len(calls) != len(expected) {
		t.Fatalf("expected %d firewall calls, got %#v", len(expected), calls)
	}
	for idx, want := range expected {
		if calls[idx] != want {
			t.Fatalf("unexpected call %d: want %q, got %q", idx, want, calls[idx])
		}
	}
}

func TestRemovePgBouncerFirewallRuleWithProgress_SkipsWhenMissing(t *testing.T) {
	var calls []string
	runFn := func(name string, args ...string) (string, error) {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		return "No rules match the specified criteria.", nil
	}

	if err := removePgBouncerFirewallRuleWithProgress(nil, runFn); err != nil {
		t.Fatalf("removePgBouncerFirewallRuleWithProgress() unexpected error = %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected only one show call when rule missing, got %#v", calls)
	}
	if !strings.Contains(calls[0], "show rule") {
		t.Fatalf("expected show-rule call, got %q", calls[0])
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
