package appcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var remoteAddrRe = regexp.MustCompile(`(?m)^\s*remote_addr\s*=\s*"(.*?)"\s*$`)

var lookupIP = net.LookupIP

type Monitor struct {
	serviceName string
}

type postgresSnapshot struct {
	Status    string  `json:"status"`
	ConnectMs float64 `json:"connect_ms"`
	QueryMs   float64 `json:"query_ms"`
	TxMs      float64 `json:"tx_ms"`
	CheckedAt string  `json:"checked_at"`
	LastError string  `json:"last_error"`
}

func NewMonitor(serviceName string) *Monitor {
	if strings.TrimSpace(serviceName) == "" {
		serviceName = defaultServiceName
	}
	return &Monitor{serviceName: serviceName}
}

func (m *Monitor) Snapshot(configPath string) StatusSnapshot {
	result := StatusSnapshot{
		ServiceName:   m.serviceName,
		ServiceState:  "unknown",
		LastCheckedAt: time.Now().Format(time.RFC3339),
	}

	serviceState, serviceErr := queryServiceState(m.serviceName)
	if serviceErr != nil {
		result.LastError = serviceErr.Error()
	} else {
		result.ServiceState = serviceState
	}

	remoteAddr, parseErr := parseRemoteAddr(configPath)
	if parseErr != nil {
		appendErr(&result.LastError, parseErr)
		result.ConnectionState = evaluateConnection(serviceState, false, false, false)
		return result
	}

	host, port, splitErr := splitHostPort(remoteAddr)
	if splitErr != nil {
		appendErr(&result.LastError, splitErr)
		result.ConnectionState = evaluateConnection(serviceState, false, false, false)
		return result
	}

	result.ServerHost = host
	result.ServerControlPort = port

	publicIP, ipErr := resolvePublicIP(host)
	if ipErr != nil {
		appendErr(&result.LastError, ipErr)
	} else {
		result.ServerPublicIP = publicIP
	}

	result.DashboardReachable = checkDashboardHealth(host, 8088)
	result.ControlPortReachable = checkTCPReachability(host, port)
	pgSnap, pgErr := fetchPostgresSnapshot(host, 8088)
	if pgErr != nil {
		appendErr(&result.LastError, pgErr)
	} else {
		result.PostgresStatus = pgSnap.Status
		result.PostgresConnectMs = formatMs(pgSnap.ConnectMs)
		result.PostgresQueryMs = formatMs(pgSnap.QueryMs)
		result.PostgresTxMs = formatMs(pgSnap.TxMs)
		result.PostgresLastChecked = pgSnap.CheckedAt
		result.PostgresLastError = strings.TrimSpace(pgSnap.LastError)
		if result.PostgresLastError != "" {
			appendErr(&result.LastError, errors.New("postgres monitor: "+result.PostgresLastError))
		}
	}
	authFailed, authErr := detectRecentAuthFailure(m.serviceName)
	if authErr != nil {
		appendErr(&result.LastError, authErr)
	}
	if authFailed {
		appendErr(&result.LastError, errors.New("autentikasi gagal: kemungkinan token client tidak cocok dengan server"))
	}
	result.ConnectionState = evaluateConnection(serviceState, result.DashboardReachable, result.ControlPortReachable, authFailed)

	return result
}

func formatMs(v float64) string {
	if v <= 0 {
		return "-"
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func parseRemoteAddr(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("gagal membaca client config: %w", err)
	}

	matches := remoteAddrRe.FindStringSubmatch(string(data))
	if len(matches) < 2 {
		return "", errors.New("remote_addr tidak ditemukan di client.toml")
	}

	remoteAddr := strings.TrimSpace(matches[1])
	if remoteAddr == "" {
		return "", errors.New("remote_addr kosong di client.toml")
	}
	return remoteAddr, nil
}

func splitHostPort(remoteAddr string) (string, string, error) {
	host, port, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return "", "", fmt.Errorf("remote_addr tidak valid (%q): %w", remoteAddr, err)
	}
	return strings.Trim(host, "[]"), port, nil
}

func resolvePublicIP(host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), nil
	}

	ips, err := lookupIP(host)
	if err != nil {
		return "", fmt.Errorf("gagal resolve host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("host %q tidak mengembalikan IP", host)
	}

	for _, ip := range ips {
		if ip.To4() != nil {
			return ip.String(), nil
		}
	}
	return ips[0].String(), nil
}

func checkDashboardHealth(host string, port int) bool {
	client := http.Client{
		Timeout: 2 * time.Second,
	}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d/health", host, port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func fetchPostgresSnapshot(host string, port int) (postgresSnapshot, error) {
	client := http.Client{
		Timeout: 2 * time.Second,
	}
	url := fmt.Sprintf("http://%s:%d/api/monitor/postgres/latest", host, port)
	resp, err := client.Get(url)
	if err != nil {
		return postgresSnapshot{}, fmt.Errorf("gagal mengambil monitor postgres: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return postgresSnapshot{}, fmt.Errorf("monitor postgres HTTP status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return postgresSnapshot{}, fmt.Errorf("gagal membaca respons monitor postgres: %w", err)
	}
	var payload postgresSnapshot
	if err := json.Unmarshal(body, &payload); err != nil {
		return postgresSnapshot{}, fmt.Errorf("gagal parse respons monitor postgres: %w", err)
	}
	return payload, nil
}

func checkTCPReachability(host, port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func evaluateConnection(serviceState string, dashboardReachable bool, controlPortReachable bool, authFailed bool) string {
	if authFailed {
		return "Auth Failed"
	}
	if strings.EqualFold(serviceState, "running") && (dashboardReachable || controlPortReachable) {
		return "Connected"
	}
	return "Disconnected"
}

func appendErr(dest *string, err error) {
	if err == nil {
		return
	}
	if *dest == "" {
		*dest = err.Error()
		return
	}
	*dest = *dest + "; " + err.Error()
}
