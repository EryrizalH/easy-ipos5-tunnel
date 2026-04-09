package appcore

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
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
	if got := evaluateConnection("running", true, false, false); got != "Connected" {
		t.Fatalf("expected Connected, got %s", got)
	}
	if got := evaluateConnection("running", false, false, false); got != "Disconnected" {
		t.Fatalf("expected Disconnected, got %s", got)
	}
	if got := evaluateConnection("stopped", true, true, false); got != "Disconnected" {
		t.Fatalf("expected Disconnected, got %s", got)
	}
	if got := evaluateConnection("running", true, true, true); got != "Auth Failed" {
		t.Fatalf("expected Auth Failed, got %s", got)
	}
}

func TestFormatMs(t *testing.T) {
	if got := formatMs(12.3456); got != "12.35" {
		t.Fatalf("unexpected formatted ms: %s", got)
	}
	if got := formatMs(-1); got != "-" {
		t.Fatalf("expected dash for invalid ms, got %s", got)
	}
}

func TestFetchPostgresSnapshot(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"Healthy","connect_ms":1.2,"query_ms":3.4,"tx_ms":4.5,"checked_at":"2026-04-09T00:00:00Z","last_error":""}`)
	}))
	t.Cleanup(ts.Close)

	host, port, err := net.SplitHostPort(ts.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}

	snap, err := fetchPostgresSnapshot(host, mustAtoi(t, port))
	if err != nil {
		t.Fatalf("fetchPostgresSnapshot error: %v", err)
	}
	if snap.Status != "Healthy" {
		t.Fatalf("unexpected status: %s", snap.Status)
	}
	if snap.QueryMs != 3.4 {
		t.Fatalf("unexpected query ms: %v", snap.QueryMs)
	}
}

func mustAtoi(t *testing.T, value string) int {
	t.Helper()
	var out int
	_, err := fmt.Sscanf(value, "%d", &out)
	if err != nil {
		t.Fatalf("atoi: %v", err)
	}
	return out
}
