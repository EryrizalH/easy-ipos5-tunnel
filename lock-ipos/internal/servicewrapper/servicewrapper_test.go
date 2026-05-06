package servicewrapper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			"--rathole-bin", `D:\bin\rathole.exe`,
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
	if cfg.RatholePath != `D:\bin\rathole.exe` {
		t.Fatalf("unexpected rathole path: %s", cfg.RatholePath)
	}
	if cfg.ServiceName != "CustomSvc" {
		t.Fatalf("unexpected service name: %s", cfg.ServiceName)
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
