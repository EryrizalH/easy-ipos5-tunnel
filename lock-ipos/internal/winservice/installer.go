package winservice

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lock-ipos/lock-ipos/internal/db"
)

const (
	DefaultServiceName = "EasyRatholeClient"
	pgBouncerService   = "PgBouncer"
	guiBinaryName      = "ipos5-rathole-gui.exe"
	pgBouncerBinary    = "pgbouncer.exe"
	pgBouncerLibEvent  = "libevent-7.dll"
	pgBouncerLibSSL    = "libssl-3-x64.dll"
	pgBouncerLibCrypto = "libcrypto-3-x64.dll"
	pgBouncerIniName   = "pgbouncer.ini"
	pgBouncerUserlist  = "userlist.txt"
	launcherFileName   = "launch-gui-admin.ps1"
	shortcutFileName   = "ipos5-rathole.lnk"
	pgBouncerHost      = "127.0.0.1"
	pgBouncerPort      = 6432
	postgresBackend    = "127.0.0.1:5444"
)

// Config controls service install/uninstall behavior.
type Config struct {
	ServiceName string
	BundleDir   string
	PGBinPath   string
}

// BundlePaths contains required sidecar files.
type BundlePaths struct {
	NSSMPath          string
	RatholePath       string
	GUIPath           string
	ClientTomlPath    string
	PgBouncerPath     string
	PgBouncerIniPath  string
	PgBouncerUserPath string
}

// GUIShortcutSpec describes launcher and shortcut artifacts for GUI.
type GUIShortcutSpec struct {
	LauncherPath      string
	ShortcutName      string
	PowerShellPath    string
	PowerShellArgs    string
	GUIExecutablePath string
}

func (c Config) normalized() Config {
	out := c
	if strings.TrimSpace(out.ServiceName) == "" {
		out.ServiceName = DefaultServiceName
	}
	if strings.TrimSpace(out.BundleDir) == "" {
		out.BundleDir = "."
	}
	return out
}

// ResolveBundlePaths validates required sidecar files in bundle directory.
func ResolveBundlePaths(bundleDir string) (BundlePaths, error) {
	cleanDir := strings.TrimSpace(bundleDir)
	if cleanDir == "" {
		return BundlePaths{}, errors.New("bundle dir tidak boleh kosong")
	}

	nssm := filepath.Join(cleanDir, "nssm.exe")
	clientToml := filepath.Join(cleanDir, "client.toml")
	guiPath := filepath.Join(cleanDir, guiBinaryName)
	pgBouncerPath := filepath.Join(cleanDir, pgBouncerBinary)
	pgBouncerLibEventPath := filepath.Join(cleanDir, pgBouncerLibEvent)
	pgBouncerLibSSLPath := filepath.Join(cleanDir, pgBouncerLibSSL)
	pgBouncerLibCryptoPath := filepath.Join(cleanDir, pgBouncerLibCrypto)
	pgBouncerIniPath := filepath.Join(cleanDir, pgBouncerIniName)
	pgBouncerUserPath := filepath.Join(cleanDir, pgBouncerUserlist)

	ratholeCandidates := []string{
		filepath.Join(cleanDir, "ipos5-rathole.exe"),
		filepath.Join(cleanDir, "rathole.exe"),
	}

	missing := make([]string, 0, 6)
	if !fileExists(nssm) {
		missing = append(missing, "nssm.exe")
	}
	if !fileExists(clientToml) {
		missing = append(missing, "client.toml")
	}
	if !fileExists(guiPath) {
		missing = append(missing, guiBinaryName)
	}
	if !fileExists(pgBouncerPath) {
		missing = append(missing, pgBouncerBinary)
	}
	if !fileExists(pgBouncerLibEventPath) {
		missing = append(missing, pgBouncerLibEvent)
	}
	if !fileExists(pgBouncerLibSSLPath) {
		missing = append(missing, pgBouncerLibSSL)
	}
	if !fileExists(pgBouncerLibCryptoPath) {
		missing = append(missing, pgBouncerLibCrypto)
	}

	rathole := ""
	for _, candidate := range ratholeCandidates {
		if fileExists(candidate) {
			rathole = candidate
			break
		}
	}
	if rathole == "" {
		missing = append(missing, "ipos5-rathole.exe/rathole.exe")
	}

	if len(missing) > 0 {
		return BundlePaths{}, fmt.Errorf("file sidecar wajib tidak lengkap di %s: %s", cleanDir, strings.Join(missing, ", "))
	}

	return BundlePaths{
		NSSMPath:          nssm,
		RatholePath:       rathole,
		GUIPath:           guiPath,
		ClientTomlPath:    clientToml,
		PgBouncerPath:     pgBouncerPath,
		PgBouncerIniPath:  pgBouncerIniPath,
		PgBouncerUserPath: pgBouncerUserPath,
	}, nil
}

// BuildInstallCommands exposes deterministic NSSM command sequence.
func BuildInstallCommands(cfg Config, paths BundlePaths, logRoot string) [][]string {
	cfg = cfg.normalized()
	return [][]string{
		{"install", cfg.ServiceName, paths.RatholePath, paths.ClientTomlPath},
		{"set", cfg.ServiceName, "AppDirectory", cfg.BundleDir},
		{"set", cfg.ServiceName, "Start", "SERVICE_AUTO_START"},
		{"set", cfg.ServiceName, "DisplayName", "IPOS5TunnelPublik Client"},
		{"set", cfg.ServiceName, "Description", "Auto-start tunnel client untuk akses publik"},
		{"set", cfg.ServiceName, "AppStdout", filepath.Join(logRoot, cfg.ServiceName+".stdout.log")},
		{"set", cfg.ServiceName, "AppStderr", filepath.Join(logRoot, cfg.ServiceName+".stderr.log")},
		{"set", cfg.ServiceName, "AppRotateFiles", "1"},
		{"set", cfg.ServiceName, "AppRotateOnline", "1"},
		{"set", cfg.ServiceName, "AppRotateSeconds", "86400"},
		{"set", cfg.ServiceName, "AppRotateBytes", "1048576"},
	}
}

// BuildPgBouncerInstallCommands exposes deterministic NSSM command sequence for PgBouncer.
func BuildPgBouncerInstallCommands(paths BundlePaths, logRoot string) [][]string {
	return [][]string{
		{"install", pgBouncerService, paths.PgBouncerPath, paths.PgBouncerIniPath},
		{"set", pgBouncerService, "AppDirectory", filepath.Dir(paths.PgBouncerPath)},
		{"set", pgBouncerService, "Start", "SERVICE_AUTO_START"},
		{"set", pgBouncerService, "DisplayName", "PgBouncer"},
		{"set", pgBouncerService, "Description", "Connection pooler PostgreSQL untuk IPOS5TunnelPublik"},
		{"set", pgBouncerService, "AppStdout", filepath.Join(logRoot, pgBouncerService+".stdout.log")},
		{"set", pgBouncerService, "AppStderr", filepath.Join(logRoot, pgBouncerService+".stderr.log")},
		{"set", pgBouncerService, "AppRotateFiles", "1"},
		{"set", pgBouncerService, "AppRotateOnline", "1"},
		{"set", pgBouncerService, "AppRotateSeconds", "86400"},
		{"set", pgBouncerService, "AppRotateBytes", "1048576"},
	}
}

// BuildUninstallCommands exposes deterministic uninstall command sequence.
func BuildUninstallCommands(cfg Config) [][]string {
	cfg = cfg.normalized()
	return [][]string{
		{"stop", cfg.ServiceName},
		{"delete", cfg.ServiceName},
	}
}

// InstallService installs and starts Windows service using nssm.
func InstallService(cfg Config) error {
	cfg = cfg.normalized()
	if !IsRunningAsAdministrator() {
		return errors.New("install service membutuhkan hak Administrator")
	}

	paths, err := ResolveBundlePaths(cfg.BundleDir)
	if err != nil {
		return err
	}

	if err := removeExistingService(cfg.ServiceName); err != nil {
		return err
	}

	programData := os.Getenv("ProgramData")
	if strings.TrimSpace(programData) == "" {
		programData = `C:\ProgramData`
	}
	logRoot := filepath.Join(programData, "easy-rathole-client", "logs")
	if err := os.MkdirAll(logRoot, 0o755); err != nil {
		return fmt.Errorf("gagal membuat folder log: %w", err)
	}

	if err := installOrUpdatePgBouncer(cfg, paths, logRoot); err != nil {
		return err
	}
	if err := waitPgBouncerHealthy(cfg.PGBinPath, 20*time.Second); err != nil {
		return fmt.Errorf("health check PgBouncer gagal: %w", err)
	}

	for _, args := range BuildInstallCommands(cfg, paths, logRoot) {
		if _, err := run(paths.NSSMPath, args...); err != nil {
			return fmt.Errorf("gagal nssm %s: %w", strings.Join(args, " "), err)
		}
	}

	if _, err := run("sc", "start", cfg.ServiceName); err != nil {
		return fmt.Errorf("gagal start service %s: %w", cfg.ServiceName, err)
	}

	if err := waitServiceState(cfg.ServiceName, "RUNNING", 20*time.Second); err != nil {
		return err
	}
	if err := setupGUIShortcut(cfg.BundleDir, paths.GUIPath); err != nil {
		return err
	}

	return nil
}

// UninstallService stops and deletes Windows service.
func UninstallService(cfg Config) error {
	cfg = cfg.normalized()
	if !IsRunningAsAdministrator() {
		return errors.New("uninstall service membutuhkan hak Administrator")
	}

	exists, err := serviceExists(cfg.ServiceName)
	if err != nil {
		return err
	}
	if exists {
		_, _ = run("sc", "stop", cfg.ServiceName)
		if _, err := run("sc", "delete", cfg.ServiceName); err != nil {
			return fmt.Errorf("gagal delete service %s: %w", cfg.ServiceName, err)
		}

		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			exists, checkErr := serviceExists(cfg.ServiceName)
			if checkErr != nil {
				return checkErr
			}
			if !exists {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if exists {
			return fmt.Errorf("service %s belum terhapus setelah timeout", cfg.ServiceName)
		}
	}
	if err := uninstallPgBouncer(); err != nil {
		return err
	}
	_ = cleanupPgBouncerArtifacts(cfg.BundleDir)
	_ = cleanupGUIShortcut(cfg.BundleDir)
	return nil
}

func installOrUpdatePgBouncer(cfg Config, paths BundlePaths, logRoot string) error {
	if strings.TrimSpace(cfg.PGBinPath) == "" {
		return errors.New("pg bin path kosong, tidak bisa verifikasi PgBouncer via psql")
	}

	if err := writePgBouncerRuntimeFiles(paths.PgBouncerIniPath, paths.PgBouncerUserPath); err != nil {
		return fmt.Errorf("gagal menyiapkan runtime file PgBouncer: %w", err)
	}
	if err := removeExistingService(pgBouncerService); err != nil {
		return fmt.Errorf("gagal menyiapkan reinstall PgBouncer: %w", err)
	}
	for _, args := range BuildPgBouncerInstallCommands(paths, logRoot) {
		if _, err := run(paths.NSSMPath, args...); err != nil {
			return fmt.Errorf("gagal nssm %s: %w", strings.Join(args, " "), err)
		}
	}
	if _, err := run("sc", "start", pgBouncerService); err != nil {
		return fmt.Errorf("gagal start service %s: %w", pgBouncerService, err)
	}
	if err := waitServiceState(pgBouncerService, "RUNNING", 20*time.Second); err != nil {
		return err
	}
	return nil
}

func waitPgBouncerHealthy(pgBinPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	address := fmt.Sprintf("%s:%d", pgBouncerHost, pgBouncerPort)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, dialErr := net.DialTimeout("tcp", address, 500*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			if err := runPgBouncerQuery(pgBinPath, "SHOW VERSION;"); err == nil {
				return nil
			} else {
				lastErr = err
			}
		} else {
			lastErr = dialErr
		}
		time.Sleep(1 * time.Second)
	}
	if lastErr != nil {
		return lastErr
	}
	return errors.New("timeout menunggu PgBouncer healthy")
}

func runPgBouncerQuery(pgBinPath, query string) error {
	psqlPath := filepath.Join(pgBinPath, "psql.exe")
	cmd := exec.Command(
		psqlPath,
		"-h", pgBouncerHost,
		"-p", fmt.Sprintf("%d", pgBouncerPort),
		"-U", db.User,
		"-d", "pgbouncer",
		"-c", query,
		"-t",
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+db.Password)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql query ke PgBouncer gagal: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func writePgBouncerRuntimeFiles(iniPath, userlistPath string) error {
	if err := os.WriteFile(iniPath, []byte(buildPgBouncerIni()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(userlistPath, []byte(buildPgBouncerUserlist()), 0o600); err != nil {
		return err
	}
	return nil
}

func buildPgBouncerIni() string {
	return strings.Join(
		[]string{
			"[databases]",
			"* = host=127.0.0.1 port=5444 dbname=postgres",
			"",
			"[pgbouncer]",
			"listen_addr = 127.0.0.1",
			"listen_port = 6432",
			"auth_type = md5",
			"auth_file = userlist.txt",
			"pool_mode = transaction",
			"max_client_conn = 300",
			"default_pool_size = 30",
			"reserve_pool_size = 10",
			"reserve_pool_timeout = 3",
			"server_reset_query = DISCARD ALL",
			"server_check_delay = 30",
			"server_idle_timeout = 600",
			"ignore_startup_parameters = extra_float_digits",
			"admin_users = " + db.User,
			"stats_users = " + db.User,
			"log_connections = 1",
			"log_disconnections = 1",
			"",
		},
		"\n",
	)
}

func buildPgBouncerUserlist() string {
	return fmt.Sprintf("\"%s\" \"%s\"\n", db.User, md5PasswordHash(db.User, db.Password))
}

func md5PasswordHash(user, password string) string {
	sum := md5.Sum([]byte(password + user))
	return "md5" + hex.EncodeToString(sum[:])
}

func uninstallPgBouncer() error {
	exists, err := serviceExists(pgBouncerService)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	_, _ = run("sc", "stop", pgBouncerService)
	if _, err := run("sc", "delete", pgBouncerService); err != nil {
		return fmt.Errorf("gagal delete service %s: %w", pgBouncerService, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		exists, checkErr := serviceExists(pgBouncerService)
		if checkErr != nil {
			return checkErr
		}
		if !exists {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("service %s belum terhapus setelah timeout", pgBouncerService)
}

func cleanupPgBouncerArtifacts(bundleDir string) error {
	var firstErr error
	for _, name := range []string{pgBouncerIniName, pgBouncerUserlist} {
		target := filepath.Join(bundleDir, name)
		if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// IsRunningAsAdministrator checks if process has local admin privileges.
func IsRunningAsAdministrator() bool {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(string(out)), "True")
}

func removeExistingService(serviceName string) error {
	exists, err := serviceExists(serviceName)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	_, _ = run("sc", "stop", serviceName)
	if _, err := run("sc", "delete", serviceName); err != nil {
		return fmt.Errorf("gagal hapus service lama %s: %w", serviceName, err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		exists, checkErr := serviceExists(serviceName)
		if checkErr != nil {
			return checkErr
		}
		if !exists {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("service lama %s belum terhapus setelah timeout", serviceName)
}

func serviceExists(serviceName string) (bool, error) {
	out, err := run("sc", "query", serviceName)
	if err != nil {
		lower := strings.ToLower(out)
		if strings.Contains(lower, "1060") || strings.Contains(lower, "does not exist") || strings.Contains(lower, "tidak ada") {
			return false, nil
		}
		return false, fmt.Errorf("gagal cek service %s: %w", serviceName, err)
	}

	if strings.Contains(strings.ToUpper(out), "SERVICE_NAME") {
		return true, nil
	}
	return false, nil
}

func waitServiceState(serviceName, state string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := run("sc", "query", serviceName)
		if err == nil && strings.Contains(strings.ToUpper(out), state) {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout menunggu service %s ke status %s", serviceName, state)
}

func run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func BuildGUIShortcutSpec(bundleDir, guiPath string) GUIShortcutSpec {
	launcherPath := filepath.Join(bundleDir, launcherFileName)
	psPath := "powershell.exe"
	psArgs := fmt.Sprintf(`-NoProfile -ExecutionPolicy Bypass -File "%s"`, launcherPath)
	return GUIShortcutSpec{
		LauncherPath:      launcherPath,
		ShortcutName:      shortcutFileName,
		PowerShellPath:    psPath,
		PowerShellArgs:    psArgs,
		GUIExecutablePath: guiPath,
	}
}

func setupGUIShortcut(bundleDir, guiPath string) error {
	spec := BuildGUIShortcutSpec(bundleDir, guiPath)
	launcherContent := buildLauncherContent(guiPath)
	if err := os.WriteFile(spec.LauncherPath, []byte(launcherContent), 0o644); err != nil {
		return fmt.Errorf("gagal menulis launcher GUI: %w", err)
	}

	createdShortcuts := make([]string, 0, 2)
	for _, desktopDir := range desktopShortcutDirs() {
		if strings.TrimSpace(desktopDir) == "" {
			continue
		}
		if _, err := os.Stat(desktopDir); err != nil {
			continue
		}
		shortcutPath := filepath.Join(desktopDir, spec.ShortcutName)
		if err := createShortcut(shortcutPath, spec.PowerShellPath, spec.PowerShellArgs, spec.GUIExecutablePath); err != nil {
			return err
		}
		createdShortcuts = append(createdShortcuts, shortcutPath)
	}

	if err := verifyGUIArtifacts(spec, createdShortcuts); err != nil {
		return err
	}

	return nil
}

func cleanupGUIShortcut(bundleDir string) error {
	var firstErr error
	spec := BuildGUIShortcutSpec(bundleDir, filepath.Join(bundleDir, guiBinaryName))
	for _, desktopDir := range desktopShortcutDirs() {
		if strings.TrimSpace(desktopDir) == "" {
			continue
		}
		shortcutPath := filepath.Join(desktopDir, spec.ShortcutName)
		if err := os.Remove(shortcutPath); err != nil && !errors.Is(err, os.ErrNotExist) && firstErr == nil {
			firstErr = err
		}
	}
	if err := os.Remove(spec.LauncherPath); err != nil && !errors.Is(err, os.ErrNotExist) && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func buildLauncherContent(guiPath string) string {
	escaped := strings.ReplaceAll(guiPath, "'", "''")
	return "$ErrorActionPreference = 'Stop'\nStart-Process -FilePath '" + escaped + "' -Verb RunAs\n"
}

func verifyGUIArtifacts(spec GUIShortcutSpec, createdShortcuts []string) error {
	if !fileExists(spec.LauncherPath) {
		return fmt.Errorf("launcher GUI tidak ditemukan setelah dibuat: %s", spec.LauncherPath)
	}
	if len(createdShortcuts) == 0 {
		return errors.New("shortcut desktop GUI tidak berhasil dibuat (desktop user/public tidak tersedia atau gagal ditulis)")
	}
	for _, shortcutPath := range createdShortcuts {
		if !fileExists(shortcutPath) {
			return fmt.Errorf("shortcut desktop GUI tidak ditemukan setelah dibuat: %s", shortcutPath)
		}
	}
	return nil
}

func desktopShortcutDirs() []string {
	userProfile := os.Getenv("USERPROFILE")
	publicProfile := os.Getenv("PUBLIC")
	candidates := []string{}
	if strings.TrimSpace(userProfile) != "" {
		candidates = append(candidates, filepath.Join(userProfile, "Desktop"))
	}
	if strings.TrimSpace(publicProfile) != "" {
		candidates = append(candidates, filepath.Join(publicProfile, "Desktop"))
	}
	return candidates
}

func createShortcut(shortcutPath, targetPath, arguments, iconPath string) error {
	command := fmt.Sprintf(
		"$w=New-Object -ComObject WScript.Shell; $s=$w.CreateShortcut('%s'); $s.TargetPath='%s'; $s.Arguments='%s'; $s.IconLocation='%s'; $s.WorkingDirectory='%s'; $s.Save()",
		escapePowerShellSingleQuoted(shortcutPath),
		escapePowerShellSingleQuoted(targetPath),
		escapePowerShellSingleQuoted(arguments),
		escapePowerShellSingleQuoted(iconPath),
		escapePowerShellSingleQuoted(filepath.Dir(iconPath)),
	)
	_, err := run("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", command)
	if err != nil {
		return fmt.Errorf("gagal membuat shortcut desktop: %w", err)
	}
	return nil
}

func escapePowerShellSingleQuoted(in string) string {
	return strings.ReplaceAll(in, "'", "''")
}
