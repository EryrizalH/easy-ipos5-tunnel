//go:build windows

package winservice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// Discovery deliberately uses the selected installation, SCM metadata, and the
// running postmaster's own files. A similarly named service is not sufficient.
func discoverPostgresTarget(pgBinPath string) (postgresTarget, error) {
	manager, err := mgr.Connect()
	if err != nil {
		return postgresTarget{}, fmt.Errorf("gagal membaca service PostgreSQL: %w", err)
	}
	defer manager.Disconnect()
	names, err := manager.ListServices()
	if err != nil {
		return postgresTarget{}, err
	}
	var matches []postgresTarget
	for _, name := range names {
		imagePath, err := postgresServiceImagePath(manager, name)
		if err != nil {
			return postgresTarget{}, fmt.Errorf("program service %s tidak dapat dikenali: %w", name, err)
		}
		if !isSelectedPostgresService(imagePath, pgBinPath) {
			continue
		}
		service, err := openPostgresService(manager, name, windows.SERVICE_QUERY_CONFIG|windows.SERVICE_QUERY_STATUS)
		if err != nil {
			return postgresTarget{}, fmt.Errorf("gagal memeriksa service PostgreSQL %s: %w", name, err)
		}
		config, configErr := service.Config()
		status, statusErr := service.Query()
		service.Close()
		if configErr != nil || statusErr != nil {
			return postgresTarget{}, fmt.Errorf("metadata service %s tidak dapat dibaca", name)
		}
		if config.StartType == mgr.StartDisabled {
			return postgresTarget{}, fmt.Errorf("service PostgreSQL %s dinonaktifkan dan tidak dapat direstart", name)
		}
		if !isSelectedPostgresService(config.BinaryPathName, pgBinPath) {
			return postgresTarget{}, fmt.Errorf("program service PostgreSQL %s berubah saat deteksi", name)
		}
		executable, args, err := postgresServiceArguments(config.BinaryPathName)
		if err != nil {
			return postgresTarget{}, fmt.Errorf("argumen service PostgreSQL %s tidak dapat dipastikan: %w", name, err)
		}
		if status.State != svc.Running {
			return postgresTarget{}, fmt.Errorf("service PostgreSQL %s belum berjalan", name)
		}
		target, err := inspectPostgresTarget(name, executable, args, status.ProcessId)
		if err != nil {
			return postgresTarget{}, fmt.Errorf("identitas PostgreSQL %s tidak pasti: %w", name, err)
		}
		matches = append(matches, target)
	}
	if len(matches) != 1 {
		return postgresTarget{}, fmt.Errorf("diperlukan tepat satu service PostgreSQL aktif pada instalasi terpilih (ditemukan %d)", len(matches))
	}
	return matches[0], nil
}

// Protected services can deny SCM queries even to an administrator. Their
// read-only registry ImagePath still identifies them as unrelated, so installing
// a database tunnel does not depend on inspecting their status or configuration.
func postgresServiceImagePath(manager *mgr.Mgr, name string) (string, error) {
	service, err := openPostgresService(manager, name, windows.SERVICE_QUERY_CONFIG)
	if err == nil {
		config, configErr := service.Config()
		service.Close()
		if configErr == nil {
			return config.BinaryPathName, nil
		}
	}
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\`+name, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()
	imagePath, _, err := key.GetStringValue("ImagePath")
	if errors.Is(err, registry.ErrNotExist) {
		// An image-less service cannot invoke the selected PostgreSQL binary.
		return "", nil
	}
	return imagePath, err
}

func isSelectedPostgresService(commandLine, pgBinPath string) bool {
	parts, err := windows.DecomposeCommandLine(commandLine)
	if err != nil || len(parts) == 0 || !samePostgresPath(filepath.Dir(parts[0]), pgBinPath) {
		return false
	}
	base := strings.ToLower(filepath.Base(parts[0]))
	return base == "pg_ctl.exe" || base == "postgres.exe"
}

func openPostgresService(manager *mgr.Mgr, name string, access uint32) (*mgr.Service, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	handle, err := windows.OpenService(manager.Handle, namePtr, access)
	if err != nil {
		return nil, err
	}
	return &mgr.Service{Name: name, Handle: handle}, nil
}

func postgresServiceArguments(commandLine string) (string, []string, error) {
	parts, err := windows.DecomposeCommandLine(commandLine)
	if err != nil || len(parts) == 0 || !filepath.IsAbs(parts[0]) {
		return "", nil, errors.New("path program service tidak valid")
	}
	executable := parts[0]
	switch strings.ToLower(filepath.Base(executable)) {
	case "postgres.exe":
		if _, err := postgresArgumentDataDir(parts[1:]); err != nil {
			return "", nil, err
		}
		return executable, parts[1:], nil
	case "pg_ctl.exe":
		var args []string
		for i := 1; i < len(parts); i++ {
			arg := parts[i]
			if arg == "-D" || arg == "--pgdata" || arg == "-o" || arg == "--options" {
				if i+1 == len(parts) {
					return "", nil, errors.New("argumen service PostgreSQL tidak lengkap")
				}
				i++
				if arg == "-o" || arg == "--options" {
					options, err := windows.DecomposeCommandLine("postgres.exe " + parts[i])
					if err != nil {
						return "", nil, err
					}
					args = append(args, options[1:]...)
				} else {
					args = append(args, "-D", parts[i])
				}
			} else if strings.HasPrefix(arg, "--pgdata=") {
				args = append(args, "-D", strings.TrimPrefix(arg, "--pgdata="))
			} else if strings.HasPrefix(arg, "-D") && len(arg) > 2 {
				args = append(args, "-D", arg[2:])
			} else if strings.HasPrefix(arg, "--options=") {
				options, err := windows.DecomposeCommandLine("postgres.exe " + strings.TrimPrefix(arg, "--options="))
				if err != nil {
					return "", nil, err
				}
				args = append(args, options[1:]...)
			} else if strings.HasPrefix(arg, "-o") && len(arg) > 2 {
				options, err := windows.DecomposeCommandLine("postgres.exe " + arg[2:])
				if err != nil {
					return "", nil, err
				}
				args = append(args, options[1:]...)
			}
		}
		if _, err := postgresArgumentDataDir(args); err != nil {
			return "", nil, err
		}
		return executable, args, nil
	default:
		return "", nil, errors.New("bukan program service PostgreSQL")
	}
}

// PostgreSQL 9.5 writes the executable in postmaster.opts without quoting it,
// even when the installation directory contains spaces. This is not an SCM
// command line: identify the executable against the selected installation
// before parsing its quoted arguments. Never guess a binary from its basename.
var postgresExecutableSuffix = regexp.MustCompile(`(?i)postgres\.exe`)

func postgresPostmasterArguments(commandLine, expectedExecutable string) (string, []string, error) {
	commandLine = strings.TrimSpace(commandLine)
	if executable, args, err := postgresServiceArguments(commandLine); err == nil && samePostgresPath(executable, expectedExecutable) {
		return executable, args, nil
	}
	if !strings.HasPrefix(commandLine, `"`) {
		for _, match := range postgresExecutableSuffix.FindAllStringIndex(commandLine, -1) {
			end := match[1]
			executable := commandLine[:end]
			if (end == len(commandLine) || commandLine[end] == ' ' || commandLine[end] == '\t') && samePostgresPath(executable, expectedExecutable) {
				return postgresServiceArguments(windows.ComposeCommandLine([]string{executable}) + commandLine[end:])
			}
		}
	}
	return "", nil, errors.New("program postmaster.opts tidak cocok dengan instalasi terpilih")
}

func postgresArgumentDataDir(args []string) (string, error) {
	var directory string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		var value string
		switch {
		case arg == "-D":
			if i+1 == len(args) {
				return "", errors.New("argumen -D tidak lengkap")
			}
			i++
			value = args[i]
		case strings.HasPrefix(arg, "-D"):
			value = arg[2:]
		case arg == "-C" || strings.HasPrefix(arg, "--single") || strings.HasPrefix(arg, "--boot") || strings.HasPrefix(arg, "--fork") || arg == "--describe-config":
			return "", errors.New("mode program PostgreSQL tidak didukung")
		default:
			continue
		}
		if directory != "" || !filepath.IsAbs(value) {
			return "", errors.New("direktori startup PostgreSQL ambigu atau relatif")
		}
		directory = value
	}
	if directory == "" {
		return "", errors.New("direktori startup PostgreSQL tidak ditemukan")
	}
	return directory, nil
}

func readPostgresConfigPaths(executable string, args []string) (string, string, error) {
	read := func(setting string) (string, error) {
		value, err := readPostgresConfigSetting(executable, args, setting)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(value) || strings.ContainsAny(value, "\r\n") {
			return "", fmt.Errorf("path %s PostgreSQL tidak absolut", setting)
		}
		return filepath.Clean(value), nil
	}
	config, err := read("config_file")
	if err != nil {
		return "", "", err
	}
	directory, err := read("data_directory")
	return config, directory, err
}

func readPostgresConfigSetting(executable string, args []string, setting string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// PostgreSQL 9.5 permits administrator/root read-only config inspection
	// only when -C is the FIRST argument. This never starts a database.
	postgres := filepath.Join(filepath.Dir(executable), "postgres.exe")
	command := exec.CommandContext(ctx, postgres, append([]string{"-C", setting}, args...)...)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("gagal membaca %s via postgres -C: %w", setting, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func validatePostgresConfiguredPort(executable string, args []string, activePort int) error {
	value, err := readPostgresConfigSetting(executable, args, "port")
	if err != nil {
		return err
	}
	return matchPostgresConfiguredPort(value, activePort)
}

func matchPostgresConfiguredPort(value string, activePort int) error {
	port, err := strconv.Atoi(value)
	if err != nil || (port != postgresLegacyPort && port != postgresBackendPort) {
		return errors.New("port konfigurasi PostgreSQL tidak valid (harus 5444 atau 5445)")
	}
	if port != activePort {
		return errors.New("perubahan port PostgreSQL masih menunggu restart; tuning dibatalkan")
	}
	return nil
}

func inspectPostgresTarget(name, executable string, args []string, servicePID uint32) (postgresTarget, error) {
	config, directory, err := readPostgresConfigPaths(executable, args)
	if err != nil {
		return postgresTarget{}, err
	}
	pidData, err := os.ReadFile(filepath.Join(directory, "postmaster.pid"))
	if err != nil {
		return postgresTarget{}, fmt.Errorf("postmaster.pid tidak dapat dibaca: %w", err)
	}
	pid, port, err := parsePostmasterIdentity(pidData, directory)
	if err != nil {
		return postgresTarget{}, err
	}
	if err := validatePostgresConfiguredPort(executable, args, port); err != nil {
		return postgresTarget{}, err
	}
	if err := verifyPostgresProcess(pid, servicePID, filepath.Join(filepath.Dir(executable), "postgres.exe")); err != nil {
		return postgresTarget{}, err
	}
	opts, err := os.ReadFile(filepath.Join(directory, "postmaster.opts"))
	if err != nil {
		return postgresTarget{}, fmt.Errorf("postmaster.opts tidak dapat dibaca: %w", err)
	}
	activeExe, activeArgs, err := postgresPostmasterArguments(string(opts), filepath.Join(filepath.Dir(executable), "postgres.exe"))
	if err != nil {
		return postgresTarget{}, err
	}
	activeConfig, activeDirectory, err := readPostgresConfigPaths(activeExe, activeArgs)
	if err != nil {
		return postgresTarget{}, err
	}
	if !samePostgresPath(config, activeConfig) || !samePostgresPath(directory, activeDirectory) {
		return postgresTarget{}, errors.New("konfigurasi service berbeda dari postmaster aktif")
	}
	if err := validatePostgresConfiguredPort(activeExe, activeArgs, port); err != nil {
		return postgresTarget{}, err
	}
	// SHOW may be denied to the application's non-superuser role. The already
	// validated SCM/postmaster metadata remains the fallback without auth edits.
	if value, err := queryPostgresTuning(filepath.Dir(executable), port, "SELECT current_setting('config_file') || chr(10) || current_setting('data_directory');"); err == nil {
		paths := strings.Split(strings.ReplaceAll(strings.TrimSpace(value), "\r\n", "\n"), "\n")
		if len(paths) != 2 || !samePostgresPath(config, paths[0]) || !samePostgresPath(directory, paths[1]) {
			return postgresTarget{}, errors.New("konfigurasi instance aktif berbeda dari metadata service")
		}
	}
	return postgresTarget{ConfigPath: config, DataDir: directory, ServiceName: name, Port: port}, nil
}

func parsePostmasterIdentity(data []byte, directory string) (uint32, int, error) {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(string(data)), "\r\n", "\n"), "\n")
	if len(lines) < 4 || !samePostgresPath(lines[1], directory) {
		return 0, 0, errors.New("direktori postmaster.pid tidak cocok")
	}
	pid, pidErr := strconv.ParseUint(strings.TrimSpace(lines[0]), 10, 32)
	port, portErr := strconv.Atoi(strings.TrimSpace(lines[3]))
	if pidErr != nil || pid == 0 || portErr != nil || (port != postgresLegacyPort && port != postgresBackendPort) {
		return 0, 0, errors.New("PID/port postmaster tidak valid (harus 5444 atau 5445)")
	}
	return uint32(pid), port, nil
}

func verifyPostgresProcess(pid, servicePID uint32, executable string) error {
	if servicePID == 0 {
		return errors.New("PID service PostgreSQL kosong")
	}
	process, err := openPostgresProcessForQuery(pid)
	if err != nil {
		return fmt.Errorf("proses postmaster tidak dapat diperiksa: %w", err)
	}
	defer windows.CloseHandle(process)
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(process, 0, &buffer[0], &size); err != nil {
		return err
	}
	if !samePostgresPath(windows.UTF16ToString(buffer[:size]), executable) {
		return errors.New("proses postmaster berasal dari instalasi lain")
	}
	if pid == servicePID {
		return nil
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	parents := make(map[uint32]uint32)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return err
	}
	for {
		parents[entry.ProcessID] = entry.ParentProcessID
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if err != windows.ERROR_NO_MORE_FILES {
				return err
			}
			break
		}
	}
	for i := 0; i < 16 && pid != 0; i++ {
		pid = parents[pid]
		if pid == servicePID {
			return nil
		}
	}
	return errors.New("postmaster bukan turunan proses service terpilih")
}

func samePostgresPath(left, right string) bool {
	canonical := func(value string) string {
		if resolved, err := filepath.EvalSymlinks(value); err == nil {
			value = resolved
		}
		return filepath.Clean(value)
	}
	return filepath.IsAbs(left) && filepath.IsAbs(right) && strings.EqualFold(canonical(left), canonical(right))
}

func systemPhysicalMemory() (uint64, error) {
	// MEMORYSTATUSEX has the same field layout on 32-bit and 64-bit Windows.
	var status struct {
		Length, MemoryLoad                                 uint32
		TotalPhys, AvailPhys, TotalPageFile, AvailPageFile uint64
		TotalVirtual, AvailVirtual, AvailExtendedVirtual   uint64
	}
	status.Length = uint32(unsafe.Sizeof(status))
	result, _, err := windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx").Call(uintptr(unsafe.Pointer(&status)))
	if result == 0 {
		return 0, fmt.Errorf("gagal mendeteksi RAM Windows: %w", err)
	}
	if status.TotalPhys == 0 {
		return 0, errors.New("RAM Windows terdeteksi nol")
	}
	return status.TotalPhys, nil
}

func restartTunedPostgres(target postgresTarget) error {
	manager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer manager.Disconnect()
	service, err := openPostgresService(manager, target.ServiceName, windows.SERVICE_QUERY_CONFIG|windows.SERVICE_QUERY_STATUS|windows.SERVICE_START|windows.SERVICE_STOP)
	if err != nil {
		return err
	}
	defer service.Close()
	config, err := service.Config()
	if err != nil {
		return err
	}
	if config.StartType == mgr.StartDisabled {
		return errors.New("service PostgreSQL dinonaktifkan dan tidak dapat direstart")
	}
	executable, args, err := postgresServiceArguments(config.BinaryPathName)
	if err != nil {
		return err
	}
	configPath, directory, err := readPostgresConfigPaths(executable, args)
	if err != nil {
		return err
	}
	if !samePostgresPath(configPath, target.ConfigPath) || !samePostgresPath(directory, target.DataDir) {
		return errors.New("identitas service PostgreSQL berubah sebelum restart")
	}
	if err := validatePostgresConfiguredPort(executable, args, target.Port); err != nil {
		return err
	}
	status, err := service.Query()
	if err != nil {
		return err
	}
	if status.State == svc.Running {
		activeTarget, err := inspectPostgresTarget(target.ServiceName, executable, args, status.ProcessId)
		if err != nil {
			return err
		}
		if activeTarget.Port != target.Port {
			return errors.New("port aktif PostgreSQL berubah sebelum restart")
		}
		if _, err := service.Control(svc.Stop); err != nil {
			return fmt.Errorf("gagal menghentikan PostgreSQL: %w", err)
		}
	} else if status.State != svc.Stopped && status.State != svc.StopPending {
		return fmt.Errorf("status PostgreSQL tidak siap restart: %d", status.State)
	}
	if err := waitTunedPostgresState(service, svc.Stopped, 60*time.Second); err != nil {
		return err
	}
	if err := service.Start(); err != nil {
		return fmt.Errorf("gagal menjalankan PostgreSQL: %w", err)
	}
	return waitTunedPostgresState(service, svc.Running, 60*time.Second)
}

func waitTunedPostgresState(service *mgr.Service, desired svc.State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		status, err := service.Query()
		if err != nil {
			return err
		}
		if status.State == desired {
			return nil
		}
		if desired == svc.Running && status.State == svc.Stopped {
			return fmt.Errorf("PostgreSQL berhenti saat startup (kode %d/%d)", status.Win32ExitCode, status.ServiceSpecificExitCode)
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("batas waktu menunggu service PostgreSQL %s (status %d)", service.Name, desired)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
