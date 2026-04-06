package appcore

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestParseRemoteAddr(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "client.toml")
	content := `[client]
remote_addr = "example.com:2333"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := parseRemoteAddr(cfgPath)
	if err != nil {
		t.Fatalf("parseRemoteAddr error: %v", err)
	}
	if got != "example.com:2333" {
		t.Fatalf("unexpected remote_addr: %q", got)
	}
}

func TestParseRemoteAddrMissing(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "client.toml")
	if err := os.WriteFile(cfgPath, []byte("[client]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := parseRemoteAddr(cfgPath)
	if err == nil {
		t.Fatal("expected error when remote_addr missing")
	}
}

func TestResolvePublicIPLiteral(t *testing.T) {
	got, err := resolvePublicIP("203.0.113.10")
	if err != nil {
		t.Fatalf("resolvePublicIP literal error: %v", err)
	}
	if got != "203.0.113.10" {
		t.Fatalf("unexpected ip: %s", got)
	}
}

func TestResolvePublicIPLookupError(t *testing.T) {
	oldLookup := lookupIP
	lookupIP = func(host string) ([]net.IP, error) {
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() {
		lookupIP = oldLookup
	})

	_, err := resolvePublicIP("bad-host.local")
	if err == nil {
		t.Fatal("expected resolve error")
	}
}

func TestEvaluateConnection(t *testing.T) {
	if got := evaluateConnection("running", true, false); got != "Connected" {
		t.Fatalf("expected Connected, got %s", got)
	}
	if got := evaluateConnection("running", false, false); got != "Disconnected" {
		t.Fatalf("expected Disconnected, got %s", got)
	}
	if got := evaluateConnection("stopped", true, true); got != "Disconnected" {
		t.Fatalf("expected Disconnected, got %s", got)
	}
}
