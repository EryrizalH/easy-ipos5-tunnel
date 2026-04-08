package winservice

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveBundlePaths_Success(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, "nssm.exe"))
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
}

func TestBuildInstallCommands(t *testing.T) {
	cfg := Config{ServiceName: "EasyRatholeClient", BundleDir: `D:\bundle`}
	paths := BundlePaths{
		NSSMPath:       `D:\bundle\nssm.exe`,
		RatholePath:    `D:\bundle\ipos5-rathole.exe`,
		GUIPath:        `D:\bundle\ipos5-rathole-gui.exe`,
		ClientTomlPath: `D:\bundle\client.toml`,
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
