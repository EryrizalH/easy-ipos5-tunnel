package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lock-ipos/lock-ipos/internal/db"
	"github.com/lock-ipos/lock-ipos/internal/logger"
	"github.com/lock-ipos/lock-ipos/internal/pgadmin"
	"github.com/lock-ipos/lock-ipos/internal/pgpath"
	tui "github.com/lock-ipos/lock-ipos/internal/tui"
	"github.com/lock-ipos/lock-ipos/internal/winservice"
)

// Application states
type state int

const (
	statePathDetect state = iota
	stateMainMenu
	stateConfirm
	stateProgress
	stateResult
)

const (
	optionInstallIPPublic  = 1
	optionInstallPgBouncer = 2
	optionUninstallService = 3
	optionLockDB           = 4
	optionUnlockDB         = 5
)

// Messages
type (
	pgPathFoundMsg struct {
		path string
		err  error
	}

	permissionCheckedMsg struct {
		canCreateDB bool
		err         error
	}

	workaroundCompletedMsg struct {
		success     bool
		canCreateDB bool
		err         error
	}

	serviceActionCompletedMsg struct {
		success bool
		message string
		err     error
	}
)

// Model is the main application model
type model struct {
	currentState state
	styles       *tui.Styles

	serviceName string
	bundleDir   string

	// Path detection
	pgBinPath      string
	pathInput      *tui.TextInput
	pathStatus     string
	pathError      bool
	pathManualMode bool

	// Permission
	currentPerm    bool
	selectedOption int
	pendingOption  int

	// Result
	successResult   bool
	resultError     string
	resultMessage   string
	progressMessage string

	// Quit flag
	quitting bool
}

// Init initializes the application
func (m *model) Init() tea.Cmd {
	return tea.Batch(
		m.findPostgreSQLBin(),
		tea.EnterAltScreen,
	)
}

// findPostgreSQLBin attempts to find PostgreSQL binary
func (m *model) findPostgreSQLBin() tea.Cmd {
	return func() tea.Msg {
		logger.Info("Searching for PostgreSQL binary in default paths...")
		path, err := pgpath.FindPostgreSQLBin()
		if err != nil {
			logger.Errorf("PostgreSQL binary not found in default paths: %v", err)
		} else {
			logger.Infof("PostgreSQL binary found at: %s", path)
		}
		return pgPathFoundMsg{path: path, err: err}
	}
}

// checkCurrentPermission checks the current permission
func (m *model) checkCurrentPermission() tea.Cmd {
	return func() tea.Msg {
		logger.Info("Checking current PostgreSQL permissions...")
		canCreateDB, err := db.GetCurrentPermission(m.pgBinPath)
		if err != nil {
			logger.Errorf("Failed to check permission: %v", err)
		} else {
			if canCreateDB {
				logger.Info("Current permission: CAN CREATE DATABASE (CREATEDB)")
			} else {
				logger.Info("Current permission: CANNOT CREATE DATABASE (NOCREATEDB)")
			}
		}
		return permissionCheckedMsg{canCreateDB: canCreateDB, err: err}
	}
}

// updatePermission updates the user's permission via workaround workflow
func (m *model) updatePermission(allowCreateDB bool) tea.Cmd {
	return func() tea.Msg {
		action := "NOCREATEDB"
		if allowCreateDB {
			action = "CREATEDB"
		}
		logger.Infof("Updating permission to: %s", action)

		err := pgadmin.SetPermissionViaWorkaround(m.pgBinPath, db.User, allowCreateDB)
		if err != nil {
			logger.Errorf("Failed to update permission via workaround: %v", err)
			return workaroundCompletedMsg{success: false, err: err}
		}

		logger.Info("Permission update command executed successfully")

		canCreateDB, err := db.GetCurrentPermission(m.pgBinPath)
		if err != nil {
			logger.Errorf("Failed to verify permission: %v", err)
			return workaroundCompletedMsg{success: false, err: err}
		}

		return workaroundCompletedMsg{success: true, canCreateDB: canCreateDB}
	}
}

func (m *model) installServiceCmd() tea.Cmd {
	return func() tea.Msg {
		cfg := winservice.Config{ServiceName: m.serviceName, BundleDir: m.bundleDir, PGBinPath: m.pgBinPath, InstallMode: winservice.InstallModeIPPublicOnly}
		if err := winservice.InstallService(cfg); err != nil {
			return serviceActionCompletedMsg{success: false, err: err}
		}
		return serviceActionCompletedMsg{success: true, message: "Install IP publik berhasil: service EasyRatholeClient terpasang. Forward DB diarahkan ke 127.0.0.1:5444 dan GUI dibuka lewat shortcut desktop 'ipos5-rathole' (UAC Run as Administrator)."}
	}
}

func (m *model) installPgBouncerCmd() tea.Cmd {
	return func() tea.Msg {
		cfg := winservice.Config{ServiceName: m.serviceName, BundleDir: m.bundleDir, PGBinPath: m.pgBinPath, InstallMode: winservice.InstallModePgBouncerOnly}
		if err := winservice.InstallService(cfg); err != nil {
			return serviceActionCompletedMsg{success: false, err: err}
		}
		return serviceActionCompletedMsg{success: true, message: "Install PgBouncer berhasil: PostgreSQL dimigrasikan ke 127.0.0.1:5445 dan PgBouncer listen di 127.0.0.1:5444. Service EasyRatholeClient tidak di-install ulang."}
	}
}

func (m *model) uninstallServiceCmd() tea.Cmd {
	return func() tea.Msg {
		cfg := winservice.Config{ServiceName: m.serviceName, BundleDir: m.bundleDir, PGBinPath: m.pgBinPath}
		if err := winservice.UninstallService(cfg); err != nil {
			return serviceActionCompletedMsg{success: false, err: err}
		}
		return serviceActionCompletedMsg{success: true, message: "Service IP Public dan PgBouncer berhasil di-uninstall, PostgreSQL dikembalikan ke port 127.0.0.1:5444, dan GUI shortcut desktop sudah dibersihkan."}
	}
}

// Update handles messages and updates the model
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case pgPathFoundMsg:
		return m.handlePathFound(msg)
	case permissionCheckedMsg:
		return m.handlePermissionChecked(msg)
	case workaroundCompletedMsg:
		return m.handleWorkaroundCompleted(msg)
	case serviceActionCompletedMsg:
		return m.handleServiceCompleted(msg)
	case tea.WindowSizeMsg:
		return m, nil
	}

	return m, nil
}

func (m *model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC || keyString(msg) == "q" {
		m.quitting = true
		return m, tea.Quit
	}

	switch m.currentState {
	case statePathDetect:
		if m.pathManualMode {
			var cmd tea.Cmd
			m.pathInput, cmd = m.pathInput.Update(msg)

			if msg.Type == tea.KeyEnter {
				inputPath := m.pathInput.GetValue()
				if inputPath != "" {
					path, err := pgpath.FindPostgreSQLBinInPath(inputPath)
					if err != nil {
						m.pathStatus = "Path tidak valid: " + err.Error()
						m.pathError = true
					} else {
						m.pgBinPath = path
						m.pathStatus = "Path ditemukan!"
						m.pathError = false
						return m, m.checkCurrentPermission()
					}
				}
			}

			if msg.Type == tea.KeyEsc {
				m.quitting = true
				return m, tea.Quit
			}

			return m, cmd
		}

		if msg.Type == tea.KeyEsc {
			m.pathManualMode = true
			m.pathStatus = "Silakan masukkan path PostgreSQL secara manual:"
			m.pathError = true
			return m, nil
		}

	case stateMainMenu:
		if keyString(msg) == "1" {
			m.selectedOption = optionInstallIPPublic
			return m, nil
		}
		if keyString(msg) == "2" {
			m.selectedOption = optionInstallPgBouncer
			return m, nil
		}
		if keyString(msg) == "3" {
			m.selectedOption = optionUninstallService
			return m, nil
		}
		if keyString(msg) == "4" {
			m.selectedOption = optionLockDB
			return m, nil
		}
		if keyString(msg) == "5" {
			m.selectedOption = optionUnlockDB
			return m, nil
		}

		if msg.Type == tea.KeyUp {
			if m.selectedOption > optionInstallIPPublic {
				m.selectedOption--
			}
			return m, nil
		}
		if msg.Type == tea.KeyDown {
			if m.selectedOption < optionUnlockDB {
				m.selectedOption++
			}
			return m, nil
		}

		if msg.Type == tea.KeyEnter && m.selectedOption > 0 {
			m.pendingOption = m.selectedOption
			m.currentState = stateConfirm
			return m, nil
		}

		return m, nil

	case stateConfirm:
		if msg.Type == tea.KeyEnter {
			m.currentState = stateProgress
			switch m.pendingOption {
			case optionInstallIPPublic:
				m.progressMessage = "Menginstall service IP Public (prepare bundle -> sinkronisasi client.toml DB 127.0.0.1:5444 -> install EasyRatholeClient)..."
				return m, m.installServiceCmd()
			case optionInstallPgBouncer:
				m.progressMessage = "Menginstall PgBouncer (migrasi PostgreSQL ke 5445 -> PgBouncer listen 5444 -> health check)..."
				return m, m.installPgBouncerCmd()
			case optionUninstallService:
				m.progressMessage = "Menguninstall service IP Public dan membersihkan PgBouncer..."
				return m, m.uninstallServiceCmd()
			case optionLockDB:
				m.progressMessage = "Menjalankan lock pembuatan database..."
				return m, m.updatePermission(false)
			case optionUnlockDB:
				m.progressMessage = "Melepas lock pembuatan database..."
				return m, m.updatePermission(true)
			default:
				m.currentState = stateMainMenu
				return m, nil
			}
		}

		if msg.Type == tea.KeyEsc {
			m.currentState = stateMainMenu
			m.pendingOption = 0
			return m, nil
		}

		return m, nil

	case stateResult:
		if msg.Type == tea.KeyEnter {
			m.currentState = stateMainMenu
			m.resultError = ""
			m.resultMessage = ""
			if m.pendingOption == optionLockDB || m.pendingOption == optionUnlockDB {
				return m, m.checkCurrentPermission()
			}
			return m, nil
		}

		return m, nil
	}

	return m, nil
}

func (m *model) handlePathFound(msg pgPathFoundMsg) (tea.Model, tea.Cmd) {
	if msg.err == nil {
		m.pgBinPath = msg.path
		m.pathStatus = "PostgreSQL ditemukan di: " + msg.path
		m.pathError = false
		m.currentState = stateMainMenu
		return m, m.checkCurrentPermission()
	}

	m.pathManualMode = true
	m.pathStatus = "PostgreSQL tidak ditemukan di lokasi default. Silakan masukkan path:"
	m.pathError = true
	m.currentState = statePathDetect
	return m, nil
}

func (m *model) handlePermissionChecked(msg permissionCheckedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.pathStatus = "Gagal koneksi database: " + msg.err.Error()
		m.pathError = true
		m.currentState = statePathDetect
		return m, nil
	}

	m.currentPerm = msg.canCreateDB
	m.currentState = stateMainMenu
	return m, nil
}

func (m *model) handleWorkaroundCompleted(msg workaroundCompletedMsg) (tea.Model, tea.Cmd) {
	m.currentState = stateResult
	if msg.success {
		m.successResult = true
		m.currentPerm = msg.canCreateDB
		m.resultError = ""
		if m.pendingOption == optionLockDB {
			m.resultMessage = "Kunci pembuatan database berhasil diaktifkan."
		} else {
			m.resultMessage = "Kunci pembuatan database berhasil dilepas."
		}
		return m, nil
	}

	m.successResult = false
	m.resultError = msg.err.Error()
	m.resultMessage = ""
	return m, nil
}

func (m *model) handleServiceCompleted(msg serviceActionCompletedMsg) (tea.Model, tea.Cmd) {
	m.currentState = stateResult
	m.successResult = msg.success
	if msg.success {
		m.resultMessage = msg.message
		m.resultError = ""
		return m, nil
	}
	m.resultMessage = ""
	m.resultError = msg.err.Error()
	return m, nil
}

// View renders the UI
func (m *model) View() string {
	if m.quitting {
		return ""
	}

	switch m.currentState {
	case statePathDetect:
		return tui.RenderPathDetect(m.styles, m.pgBinPath, m.pathInput.GetValue(), m.pathInput, m.pathStatus, m.pathError)
	case stateMainMenu:
		return tui.RenderMainMenu(m.styles, m.currentPerm, m.selectedOption)
	case stateConfirm:
		return tui.RenderConfirm(m.styles, m.pendingOption, m.currentPerm, m.serviceName, m.bundleDir)
	case stateProgress:
		return tui.RenderProgress(m.styles, m.progressMessage)
	case stateResult:
		return tui.RenderResult(m.styles, m.successResult, m.currentPerm, m.pendingOption, m.resultMessage, m.resultError)
	default:
		return "Unknown state"
	}
}

// keyString returns the string representation of a key press
func keyString(msg tea.KeyMsg) string {
	if len(msg.Runes) > 0 {
		return string(msg.Runes[0])
	}
	s := msg.String()
	if len(s) > 0 {
		return s
	}
	return ""
}

func defaultBundleDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exePath)
}

func main() {
	serviceName := flag.String("service-name", winservice.DefaultServiceName, "Windows service name")
	bundleDir := flag.String("bundle-dir", defaultBundleDir(), "Directory containing sidecar files (nssm.exe, pgbouncer.exe, libevent-7.dll, libssl-3-x64.dll, libcrypto-3-x64.dll, ipos5-rathole.exe/rathole.exe, client.toml)")
	flag.Parse()

	if err := logger.Init(); err != nil {
		fmt.Printf("Warning: Could not initialize logger: %v\n", err)
	}
	defer logger.Close()

	if !isTerminalCompatible() {
		fmt.Println("Error: This application requires a compatible terminal.")
		fmt.Println("Please run from Windows Terminal, PowerShell, or Command Prompt.")
		logger.Error("Terminal not compatible")
		os.Exit(1)
	}

	styles := tui.DefaultStyles()

	m := &model{
		currentState:    statePathDetect,
		styles:          styles,
		serviceName:     *serviceName,
		bundleDir:       *bundleDir,
		pathInput:       tui.NewTextInput("Masukkan path PostgreSQL...", 50),
		pathStatus:      "Mencari PostgreSQL...",
		pathError:       false,
		selectedOption:  optionInstallIPPublic,
		quitting:        false,
		progressMessage: "Memproses...",
	}

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		logger.Errorf("Program error: %v", err)
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}

// isTerminalCompatible checks if the terminal is compatible
func isTerminalCompatible() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
