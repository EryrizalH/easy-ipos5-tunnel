package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lock-ipos/lock-ipos/internal/db"
	"github.com/lock-ipos/lock-ipos/internal/logger"
	"github.com/lock-ipos/lock-ipos/internal/pgadmin"
	"github.com/lock-ipos/lock-ipos/internal/pgpath"
	"github.com/lock-ipos/lock-ipos/internal/progress"
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
	spinnerTickMsg struct{}

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

const progressLogLimit = 12

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type progressStep struct {
	ID     string
	Label  string
	Status progress.StepStatus
	Detail string
}

type progressStateData struct {
	Title        string
	Steps        []progressStep
	stepIndex    map[string]int
	Logs         *progress.LineBuffer
	Summary      string
	StartedAt    time.Time
	SpinnerFrame int
}

func newProgressStateData(title string, steps []progress.StepDefinition) *progressStateData {
	data := &progressStateData{
		Title:     title,
		Steps:     make([]progressStep, 0, len(steps)),
		stepIndex: make(map[string]int, len(steps)),
		Logs:      progress.NewLineBuffer(progressLogLimit),
		StartedAt: time.Now(),
	}
	for _, step := range steps {
		data.stepIndex[step.ID] = len(data.Steps)
		data.Steps = append(data.Steps, progressStep{ID: step.ID, Label: step.Label, Status: progress.StepPending})
	}
	return data
}

func (p *progressStateData) ensureStep(id, label string) int {
	if p == nil {
		return -1
	}
	if idx, ok := p.stepIndex[id]; ok {
		if label != "" && p.Steps[idx].Label == "" {
			p.Steps[idx].Label = label
		}
		return idx
	}
	idx := len(p.Steps)
	p.stepIndex[id] = idx
	p.Steps = append(p.Steps, progressStep{ID: id, Label: label, Status: progress.StepPending})
	return idx
}

func (p *progressStateData) startStep(msg progress.StepStartedMsg) {
	idx := p.ensureStep(msg.ID, msg.Label)
	if idx < 0 {
		return
	}
	p.Steps[idx].Status = progress.StepRunning
	if msg.Detail != "" {
		p.Steps[idx].Detail = msg.Detail
	}
}

func (p *progressStateData) finishStep(msg progress.StepFinishedMsg) {
	idx := p.ensureStep(msg.ID, msg.Label)
	if idx < 0 {
		return
	}
	if msg.Success {
		p.Steps[idx].Status = progress.StepSuccess
	} else {
		p.Steps[idx].Status = progress.StepFailed
	}
	if msg.Detail != "" {
		p.Steps[idx].Detail = msg.Detail
	}
}

func (p *progressStateData) elapsed() string {
	if p == nil {
		return "0s"
	}
	return time.Since(p.StartedAt).Round(time.Second).String()
}

func (p *progressStateData) spinner() string {
	if p == nil || len(spinnerFrames) == 0 {
		return ""
	}
	return spinnerFrames[p.SpinnerFrame%len(spinnerFrames)]
}

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
	successResult bool
	resultError   string
	resultMessage string
	progressData  *progressStateData
	progressCh    chan any

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
	case progress.StepStartedMsg:
		if m.progressData != nil {
			m.progressData.startStep(msg)
		}
		return m, waitForProgress(m.progressCh)
	case progress.StepFinishedMsg:
		if m.progressData != nil {
			m.progressData.finishStep(msg)
		}
		return m, waitForProgress(m.progressCh)
	case progress.LogLineMsg:
		if m.progressData != nil {
			m.progressData.Logs.Append(msg.Line)
		}
		return m, waitForProgress(m.progressCh)
	case progress.SummaryUpdatedMsg:
		if m.progressData != nil {
			m.progressData.Summary = msg.Summary
		}
		return m, waitForProgress(m.progressCh)
	case workaroundCompletedMsg:
		return m.handleWorkaroundCompleted(msg)
	case serviceActionCompletedMsg:
		return m.handleServiceCompleted(msg)
	case spinnerTickMsg:
		if m.currentState == stateProgress && m.progressData != nil {
			m.progressData.SpinnerFrame = (m.progressData.SpinnerFrame + 1) % len(spinnerFrames)
			return m, spinnerTick()
		}
		return m, nil
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
			return m, m.beginProgressFlow(m.pendingOption)
		}

		if msg.Type == tea.KeyEsc {
			m.currentState = stateMainMenu
			m.pendingOption = 0
			return m, nil
		}

		return m, nil

	case stateProgress:
		return m, nil

	case stateResult:
		if msg.Type == tea.KeyEnter {
			m.currentState = stateMainMenu
			m.resultError = ""
			m.resultMessage = ""
			m.progressData = nil
			m.progressCh = nil
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
	m.progressCh = nil
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
	m.progressCh = nil
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

func (m *model) beginProgressFlow(option int) tea.Cmd {
	title, steps := progressPlan(option)
	m.currentState = stateProgress
	m.progressData = newProgressStateData(title, steps)
	m.progressCh = make(chan any, 256)
	go m.runProgressWorkflow(option, m.progressCh)
	return tea.Batch(waitForProgress(m.progressCh), spinnerTick())
}

func (m *model) runProgressWorkflow(option int, ch chan any) {
	defer close(ch)
	reporter := progress.NewChannelReporter(ch)

	sendServiceResult := func(success bool, message string, err error) {
		ch <- serviceActionCompletedMsg{success: success, message: message, err: err}
	}
	sendPermissionResult := func(success bool, canCreateDB bool, err error) {
		ch <- workaroundCompletedMsg{success: success, canCreateDB: canCreateDB, err: err}
	}

	switch option {
	case optionInstallIPPublic:
		cfg := winservice.Config{ServiceName: m.serviceName, BundleDir: m.bundleDir, PGBinPath: m.pgBinPath, InstallMode: winservice.InstallModeIPPublicOnly}
		if err := winservice.InstallServiceWithProgress(cfg, reporter); err != nil {
			sendServiceResult(false, "", err)
			return
		}
		summary := "Install IP publik berhasil: service EasyRatholeClient terpasang. Forward DB diarahkan ke 127.0.0.1:5444 dan GUI dibuka lewat shortcut desktop 'ipos5-rathole' (UAC Run as Administrator)."
		reporter.Summary(summary)
		sendServiceResult(true, summary, nil)
	case optionInstallPgBouncer:
		cfg := winservice.Config{ServiceName: m.serviceName, BundleDir: m.bundleDir, PGBinPath: m.pgBinPath, InstallMode: winservice.InstallModePgBouncerOnly}
		if err := winservice.InstallServiceWithProgress(cfg, reporter); err != nil {
			sendServiceResult(false, "", err)
			return
		}
		summary := "Install PgBouncer berhasil: PostgreSQL dimigrasikan ke 127.0.0.1:5445 dan PgBouncer listen di 127.0.0.1:5444. Service EasyRatholeClient tidak di-install ulang."
		reporter.Summary(summary)
		sendServiceResult(true, summary, nil)
	case optionUninstallService:
		cfg := winservice.Config{ServiceName: m.serviceName, BundleDir: m.bundleDir, PGBinPath: m.pgBinPath}
		if err := winservice.UninstallServiceWithProgress(cfg, reporter); err != nil {
			sendServiceResult(false, "", err)
			return
		}
		summary := "Service IP Public dan PgBouncer berhasil di-uninstall, PostgreSQL dikembalikan ke port 127.0.0.1:5444, dan GUI shortcut desktop sudah dibersihkan."
		reporter.Summary(summary)
		sendServiceResult(true, summary, nil)
	case optionLockDB, optionUnlockDB:
		allowCreateDB := option == optionUnlockDB
		if err := pgadmin.SetPermissionViaWorkaroundWithProgress(m.pgBinPath, db.User, allowCreateDB, reporter); err != nil {
			sendPermissionResult(false, false, err)
			return
		}
		canCreateDB, err := db.GetCurrentPermission(m.pgBinPath)
		if err != nil {
			sendPermissionResult(false, false, err)
			return
		}
		if allowCreateDB {
			reporter.Summary("Kunci pembuatan database berhasil dilepas.")
		} else {
			reporter.Summary("Kunci pembuatan database berhasil diaktifkan.")
		}
		sendPermissionResult(true, canCreateDB, nil)
	default:
		sendServiceResult(false, "", fmt.Errorf("opsi aksi tidak dikenali: %d", option))
	}
}

func progressPlan(option int) (string, []progress.StepDefinition) {
	switch option {
	case optionInstallIPPublic:
		return "Install IP Public", []progress.StepDefinition{
			{ID: "validate-admin", Label: "Validasi hak Administrator"},
			{ID: "resolve-bundle", Label: "Validasi bundle installer"},
			{ID: "prepare-log-dir", Label: "Menyiapkan folder log runtime"},
			{ID: "sync-client-config", Label: "Sinkronisasi client.toml DB"},
			{ID: "legacy-pgbouncer", Label: "Migrasi PgBouncer lama bila ada"},
			{ID: "remove-tunnel-service", Label: "Menyiapkan reinstall service tunnel"},
			{ID: "install-tunnel-service", Label: "Install/update EasyRatholeClient"},
			{ID: "wait-tunnel-running", Label: "Menunggu service RUNNING"},
			{ID: "setup-gui-shortcut", Label: "Menyiapkan shortcut GUI"},
		}
	case optionInstallPgBouncer:
		return "Install PgBouncer", []progress.StepDefinition{
			{ID: "validate-admin", Label: "Validasi hak Administrator"},
			{ID: "resolve-bundle", Label: "Validasi bundle PgBouncer"},
			{ID: "prepare-log-dir", Label: "Menyiapkan folder log runtime"},
			{ID: "sync-client-config", Label: "Sinkronisasi client.toml DB"},
			{ID: "migrate-postgres-port", Label: "Migrasi port PostgreSQL ke 5445"},
			{ID: "preflight-pgbouncer", Label: "Preflight dependency PgBouncer"},
			{ID: "prepare-pgbouncer-runtime", Label: "Menyiapkan file runtime PgBouncer"},
			{ID: "install-pgbouncer-service", Label: "Install/update service PgBouncer"},
			{ID: "wait-pgbouncer-running", Label: "Menunggu service PgBouncer RUNNING"},
			{ID: "health-check-pgbouncer", Label: "Health check PgBouncer"},
		}
	case optionUninstallService:
		return "Uninstall Service", []progress.StepDefinition{
			{ID: "validate-admin", Label: "Validasi hak Administrator"},
			{ID: "remove-tunnel-service", Label: "Menghapus EasyRatholeClient"},
			{ID: "remove-pgbouncer-service", Label: "Menghapus service PgBouncer"},
			{ID: "rollback-postgres-port", Label: "Mengembalikan port PostgreSQL ke 5444"},
			{ID: "cleanup-artifacts", Label: "Membersihkan artefak runtime dan shortcut"},
		}
	case optionLockDB:
		return "Kunci Pembuatan Database", []progress.StepDefinition{
			{ID: "find-pghba", Label: "Mencari pg_hba.conf"},
			{ID: "backup-pghba", Label: "Membuat backup pg_hba.conf"},
			{ID: "set-trust", Label: "Mengubah auth menjadi trust"},
			{ID: "restart-trust", Label: "Restart PostgreSQL untuk mode trust"},
			{ID: "alter-user", Label: "Menjalankan ALTER USER NOCREATEDB"},
			{ID: "verify-permission", Label: "Verifikasi permission database"},
			{ID: "restore-pghba", Label: "Mengembalikan pg_hba.conf"},
			{ID: "restart-cleanup", Label: "Restart PostgreSQL untuk cleanup"},
		}
	case optionUnlockDB:
		return "Lepas Kunci Database", []progress.StepDefinition{
			{ID: "find-pghba", Label: "Mencari pg_hba.conf"},
			{ID: "backup-pghba", Label: "Membuat backup pg_hba.conf"},
			{ID: "set-trust", Label: "Mengubah auth menjadi trust"},
			{ID: "restart-trust", Label: "Restart PostgreSQL untuk mode trust"},
			{ID: "alter-user", Label: "Menjalankan ALTER USER CREATEDB"},
			{ID: "verify-permission", Label: "Verifikasi permission database"},
			{ID: "restore-pghba", Label: "Mengembalikan pg_hba.conf"},
			{ID: "restart-cleanup", Label: "Restart PostgreSQL untuk cleanup"},
		}
	default:
		return "Memproses", nil
	}
}

func waitForProgress(ch <-chan any) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func spinnerTick() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
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
		if m.progressData == nil {
			return tui.RenderProgress(m.styles, tui.ProgressViewModel{Title: "Memproses..."})
		}
		return tui.RenderProgress(m.styles, tui.ProgressViewModel{
			Title:   m.progressData.Title,
			Spinner: m.progressData.spinner(),
			Elapsed: m.progressData.elapsed(),
			Summary: m.progressData.Summary,
			Steps: func() []tui.ProgressStepView {
				steps := make([]tui.ProgressStepView, 0, len(m.progressData.Steps))
				for _, step := range m.progressData.Steps {
					steps = append(steps, tui.ProgressStepView{Label: step.Label, Status: string(step.Status), Detail: step.Detail})
				}
				return steps
			}(),
			Logs: m.progressData.Logs.Lines(),
		})
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
		currentState:   statePathDetect,
		styles:         styles,
		serviceName:    *serviceName,
		bundleDir:      *bundleDir,
		pathInput:      tui.NewTextInput("Masukkan path PostgreSQL...", 50),
		pathStatus:     "Mencari PostgreSQL...",
		pathError:      false,
		selectedOption: optionInstallIPPublic,
		quitting:       false,
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
