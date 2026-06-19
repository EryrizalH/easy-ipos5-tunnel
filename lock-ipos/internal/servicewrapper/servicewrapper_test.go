package servicewrapper

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseArgs_DefaultsFromExecutableDir(t *testing.T) {
	cfg, err := ParseArgs(nil, `D:\bundle`)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if cfg.BundleDir != `D:\bundle` {
		t.Fatalf("unexpected bundle dir: %s", cfg.BundleDir)
	}
	if cfg.ConfigPath != filepath.Join(`D:\bundle`, DefaultConfigName) {
		t.Fatalf("unexpected config path: %s", cfg.ConfigPath)
	}
	if cfg.RatholePath != filepath.Join(`D:\bundle`, DefaultRatholeBinaryName) {
		t.Fatalf("unexpected rathole path: %s", cfg.RatholePath)
	}
	if cfg.ServiceName != DefaultServiceName {
		t.Fatalf("unexpected service name: %s", cfg.ServiceName)
	}
}

func TestParseArgs_ExplicitOverrides(t *testing.T) {
	cfg, err := ParseArgs(
		[]string{
			"--bundle-dir", `D:\vpn`,
			"--config", `D:\cfg\custom.toml`,
			"--rathole-bin", `D:\bin\nusatunnel.exe`,
			"--service-name", "CustomSvc",
		},
		`D:\ignored`,
	)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if cfg.BundleDir != `D:\vpn` {
		t.Fatalf("unexpected bundle dir: %s", cfg.BundleDir)
	}
	if cfg.ConfigPath != `D:\cfg\custom.toml` {
		t.Fatalf("unexpected config path: %s", cfg.ConfigPath)
	}
	if cfg.RatholePath != `D:\bin\nusatunnel.exe` {
		t.Fatalf("unexpected rathole path: %s", cfg.RatholePath)
	}
	if cfg.ServiceName != "CustomSvc" {
		t.Fatalf("unexpected service name: %s", cfg.ServiceName)
	}
}

func TestParseArgs_TrimsNSSMQuotedPathArgumentsWithSpaces(t *testing.T) {
	cfg, err := ParseArgs(
		[]string{
			"--bundle-dir", `"C:\Program Files\NusaTunnel"`,
			"--config", `"C:\Program Files\NusaTunnel\client.toml"`,
			"--rathole-bin", `"C:\Program Files\NusaTunnel\nusatunnel.exe"`,
			"--service-name", "NusaTunnelClient",
		},
		`C:\ignored`,
	)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if cfg.BundleDir != `C:\Program Files\NusaTunnel` {
		t.Fatalf("unexpected bundle dir: %s", cfg.BundleDir)
	}
	if cfg.ConfigPath != `C:\Program Files\NusaTunnel\client.toml` {
		t.Fatalf("unexpected config path: %s", cfg.ConfigPath)
	}
	if cfg.RatholePath != `C:\Program Files\NusaTunnel\nusatunnel.exe` {
		t.Fatalf("unexpected rathole path: %s", cfg.RatholePath)
	}
}

func TestValidate_MissingRatholeBinary(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, DefaultConfigName)
	if err := os.WriteFile(configPath, []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err := Config{
		BundleDir:   tmp,
		ConfigPath:  configPath,
		RatholePath: filepath.Join(tmp, DefaultRatholeBinaryName),
	}.Validate()
	if err == nil {
		t.Fatal("expected missing rathole binary validation error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "rathole") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_MissingConfig(t *testing.T) {
	tmp := t.TempDir()
	ratholePath := filepath.Join(tmp, DefaultRatholeBinaryName)
	if err := os.WriteFile(ratholePath, []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err := Config{
		BundleDir:   tmp,
		ConfigPath:  filepath.Join(tmp, DefaultConfigName),
		RatholePath: ratholePath,
	}.Validate()
	if err == nil {
		t.Fatal("expected missing config validation error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "client.toml") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_ReturnsErrorWhenRatholeExitsZeroAfterGrace(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, DefaultConfigName)
	if err := os.WriteFile(configPath, []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	helperPath := filepath.Join(tmp, "exit-zero-helper.exe")
	helperSource := filepath.Join(tmp, "exit-zero-helper.go")
	helperCode := `package main

import "time"

func main() {
	time.Sleep(300 * time.Millisecond)
}
`
	if err := os.WriteFile(helperSource, []byte(helperCode), 0o644); err != nil {
		t.Fatalf("WriteFile(helper) error = %v", err)
	}
	if out, err := exec.Command("go", "build", "-o", helperPath, helperSource).CombinedOutput(); err != nil {
		t.Fatalf("go build helper failed: %v: %s", err, strings.TrimSpace(string(out)))
	}

	var stdout bytes.Buffer
	err := Run(context.Background(), Config{
		BundleDir:          tmp,
		ConfigPath:         configPath,
		RatholePath:        helperPath,
		StartupGracePeriod: time.Nanosecond,
	}, &stdout, nil)
	if err == nil {
		t.Fatal("expected error when child exits cleanly after startup grace")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "tanpa error") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "terlalu cepat") {
		t.Fatalf("expected startup grace to elapse before exit, got early-exit error: %v", err)
	}
}
