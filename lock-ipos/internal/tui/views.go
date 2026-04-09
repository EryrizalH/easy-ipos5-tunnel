package tui

import (
	"fmt"
	"strings"

	"github.com/lock-ipos/lock-ipos/internal/db"
)

type ProgressStepView struct {
	Label  string
	Status string
	Detail string
}

type ProgressViewModel struct {
	Title   string
	Spinner string
	Elapsed string
	Summary string
	Steps   []ProgressStepView
	Logs    []string
}

// RenderPathDetect renders the path detection screen
func RenderPathDetect(styles *Styles, pgBinPath string, testingPath string, input *TextInput, status string, isError bool) string {
	var b strings.Builder

	b.WriteString(styles.Box.Render(
		styles.Title.Render("PostgreSQL Permission Manager") +
			"\n\n" +
			styles.Subtitle.Render("Mencari PostgreSQL Binary...") +
			"\n\n" +
			renderStatus(styles, status, isError) +
			"\n\n" +
			styles.NormalText.Render("Lokasi default yang dicek:") +
			"\n" +
			styles.MutedText.Render("  1. C:\\Program Files (x86)\\Inspirasibiz\\Server System 1.0\\pgsql9.5\\bin") +
			"\n" +
			styles.MutedText.Render("  2. D:\\Server System 1.0\\pgsql9.5\\bin") +
			"\n\n" +
			styles.NormalText.Render("Path PostgreSQL: ") +
			"\n" +
			input.View(styles) +
			"\n\n" +
			styles.HelpText.Render("[Enter] Gunakan path ini  [Esc] Keluar"),
	))

	return b.String()
}

// RenderMainMenu renders the main menu screen
func RenderMainMenu(styles *Styles, canCreateDB bool, selectedOption int) string {
	var b strings.Builder

	var statusText, statusIcon string
	if canCreateDB {
		statusText = "BOLEH Buat Database"
		statusIcon = "✓"
	} else {
		statusText = "TIDAK BOLEH Buat Database"
		statusIcon = "✗"
	}

	menuOptions := []struct {
		number   int
		label    string
		details  string
		selected bool
	}{
		{number: 1, label: "Install IP Public", details: "(EasyRatholeClient, tanpa PgBouncer)", selected: selectedOption == 1},
		{number: 2, label: "Install PgBouncer", details: "(Meningkatkan performa)", selected: selectedOption == 2},
		{number: 3, label: "Uninstall Service IP Public", details: "(EasyRatholeClient + PgBouncer)", selected: selectedOption == 3},
		{number: 4, label: "Kunci Pembuatan Database Baru", details: "(NOCREATEDB)", selected: selectedOption == 4},
		{number: 5, label: "Lepas Kunci Database Baru", details: "(CREATEDB)", selected: selectedOption == 5},
	}

	var menuText strings.Builder
	menuText.WriteString(styles.NormalText.Render("Pilih Aksi:"))
	menuText.WriteString("\n\n")

	for _, opt := range menuOptions {
		var line string
		if opt.selected {
			line = "▶ " +
				styles.MenuItemActive.Render(
					fmt.Sprintf("[%d] ", opt.number)+
						opt.label+" "+
						styles.MutedText.Render(opt.details),
				)
		} else {
			line = "  " +
				styles.MenuItem.Render(
					fmt.Sprintf("[%d] ", opt.number)+
						opt.label+" "+
						styles.MutedText.Render(opt.details),
				)
		}
		menuText.WriteString(line)
		menuText.WriteString("\n")
	}

	menuText.WriteString("\n")
	menuText.WriteString(styles.StatusLabel.Render("Status DB: "))
	if canCreateDB {
		menuText.WriteString(styles.StatusAllow.Render(statusIcon + " " + statusText))
	} else {
		menuText.WriteString(styles.StatusLock.Render(statusIcon + " " + statusText))
	}

	menuText.WriteString("\n\n")
	menuText.WriteString(styles.HelpText.Render("[↑/↓] Pilih  [1..5] Pilih  [Enter] Konfirmasi  [Q] Keluar"))

	b.WriteString(styles.Box.Render(
		styles.Title.Render("IPOS5 Unified Tools") +
			"\n\n" +
			menuText.String(),
	))

	return b.String()
}

// RenderConfirm renders the confirmation screen
func RenderConfirm(styles *Styles, option int, canCreateDB bool, serviceName, bundleDir string) string {
	var b strings.Builder

	var actionTitle, actionDesc, consequence string
	var detailLines []string

	switch option {
	case 1:
		actionTitle = "Install IP Public"
		actionDesc = "Anda akan menginstall/update service tunnel publik tanpa install ulang PgBouncer."
		consequence = "Installer akan memastikan DB forwarding local_addr=127.0.0.1:5444 lalu membuat service tunnel."
		detailLines = []string{
			"Service Name: " + serviceName,
			"Bundle Dir : " + bundleDir,
			"Wajib ada  : nssm.exe, ipos5-rathole.exe/rathole.exe, ipos5-rathole-gui.exe, client.toml",
		}
	case 2:
		actionTitle = "Install PgBouncer"
		actionDesc = "Anda akan install/update service PgBouncer untuk meningkatkan performa koneksi PostgreSQL."
		consequence = "Installer akan migrasi PostgreSQL ke 127.0.0.1:5445, lalu PgBouncer listen di 127.0.0.1:5444."
		detailLines = []string{
			"Service Name: PgBouncer",
			"Bundle Dir : " + bundleDir,
			"Wajib ada  : pgbouncer.exe, libevent-7.dll, libssl-3-x64.dll, libcrypto-3-x64.dll, libwinpthread-1.dll, psql.exe",
		}
	case 3:
		actionTitle = "Uninstall Service IP Public"
		actionDesc = "Anda akan menghapus service tunnel publik."
		consequence = "Service EasyRatholeClient dan PgBouncer akan dihentikan lalu dihapus dari Windows Service, kemudian PostgreSQL dikembalikan ke port 127.0.0.1:5444."
		detailLines = []string{
			"Service Name: " + serviceName,
		}
	case 4:
		actionTitle = "Kunci Pembuatan Database"
		actionDesc = "Anda akan MENGLOCK permission pembuatan database."
		consequence = "User tidak akan bisa membuat database baru."
		detailLines = []string{db.GetSQLCommandForPreview(false)}
	case 5:
		actionTitle = "Lepas Kunci Database"
		actionDesc = "Anda akan membuka lock permission pembuatan database."
		consequence = "User akan bisa membuat database baru lagi."
		detailLines = []string{db.GetSQLCommandForPreview(true)}
	default:
		return ""
	}

	var content strings.Builder
	content.WriteString(styles.Subtitle.Render(actionTitle))
	content.WriteString("\n\n")
	content.WriteString(styles.NormalText.Render(actionDesc))
	content.WriteString("\n\n")

	content.WriteString(styles.NormalText.Render("Detail:"))
	content.WriteString("\n")
	for _, line := range detailLines {
		content.WriteString(styles.CodeBox.Render(line))
		content.WriteString("\n")
	}

	content.WriteString(styles.NormalText.Render(consequence))
	content.WriteString("\n\n")
	content.WriteString(styles.HelpText.Render("[Enter] Lanjutkan  [Esc] Batal"))

	b.WriteString(styles.Box.Render(content.String()))

	return b.String()
}

// RenderProgress renders the progress/execution screen
func RenderProgress(styles *Styles, vm ProgressViewModel) string {
	var b strings.Builder
	var content strings.Builder
	if strings.TrimSpace(vm.Title) == "" {
		vm.Title = "Memproses..."
	}
	content.WriteString(styles.Subtitle.Render(vm.Title))
	content.WriteString("\n\n")
	spinner := vm.Spinner
	if strings.TrimSpace(spinner) == "" {
		spinner = "•"
	}
	content.WriteString(styles.ProgressRunning.Render(fmt.Sprintf("%s Proses berjalan", spinner)))
	content.WriteString("\n")
	if strings.TrimSpace(vm.Elapsed) != "" {
		content.WriteString(styles.MutedText.Render("Durasi: " + vm.Elapsed))
		content.WriteString("\n")
	}
	if strings.TrimSpace(vm.Summary) != "" {
		content.WriteString(styles.MutedText.Render(vm.Summary))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(styles.NormalText.Render("Tahapan:"))
	content.WriteString("\n")
	for _, step := range vm.Steps {
		icon := "•"
		style := styles.ProgressPending
		switch strings.ToLower(strings.TrimSpace(step.Status)) {
		case "running":
			icon = "▶"
			style = styles.ProgressRunning
		case "success":
			icon = "✓"
			style = styles.ProgressSuccess
		case "failed":
			icon = "✗"
			style = styles.ProgressFailed
		default:
			icon = "•"
		}

		line := fmt.Sprintf("%s %s", icon, step.Label)
		if strings.TrimSpace(step.Detail) != "" {
			line += " — " + step.Detail
		}
		content.WriteString(style.Render(line))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(styles.NormalText.Render("Log terbaru:"))
	content.WriteString("\n")
	if len(vm.Logs) == 0 {
		content.WriteString(styles.LogBox.Render(styles.MutedText.Render("Belum ada log progress...")))
	} else {
		content.WriteString(styles.LogBox.Render(styles.LogText.Render(strings.Join(vm.Logs, "\n"))))
	}
	content.WriteString("\n\n")
	content.WriteString(styles.HelpText.Render("[Q] Keluar"))

	b.WriteString(styles.Box.Render(content.String()))
	return b.String()
}

// RenderResult renders the result screen
func RenderResult(styles *Styles, success bool, canCreateDB bool, action int, resultMsg, errorMsg string) string {
	var b strings.Builder
	var content strings.Builder

	if success {
		content.WriteString(styles.SuccessText.Render("Berhasil!"))
		content.WriteString("\n\n")
		if strings.TrimSpace(resultMsg) != "" {
			content.WriteString(styles.NormalText.Render(resultMsg))
		} else {
			content.WriteString(styles.NormalText.Render("Aksi selesai tanpa error."))
		}
	} else {
		content.WriteString(styles.ErrorText.Render("Gagal!"))
		content.WriteString("\n\n")
		content.WriteString(styles.NormalText.Render("Terjadi kesalahan saat menjalankan aksi."))
		if strings.TrimSpace(errorMsg) != "" {
			if len(errorMsg) > 420 {
				errorMsg = errorMsg[:417] + "..."
			}
			content.WriteString("\n\n")
			content.WriteString(styles.MutedText.Render("Error: " + errorMsg))
			content.WriteString("\n")
			content.WriteString(styles.HelpText.Render("Tip: cek log.txt (folder setup.exe) dan log service di C:\\ProgramData\\easy-rathole-client\\logs"))
		}
	}

	if action == 4 || action == 5 {
		content.WriteString("\n\n")
		content.WriteString(styles.StatusLabel.Render("Status DB: "))
		if canCreateDB {
			content.WriteString(styles.StatusAllow.Render("✓ BOLEH Buat Database"))
		} else {
			content.WriteString(styles.StatusLock.Render("✗ TIDAK BOLEH Buat Database"))
		}
	}

	content.WriteString("\n\n")
	content.WriteString(styles.HelpText.Render("[Enter] Kembali ke Menu  [Q] Keluar"))

	b.WriteString(styles.Box.Render(content.String()))
	return b.String()
}

// renderStatus renders a status message with appropriate styling
func renderStatus(styles *Styles, message string, isError bool) string {
	if message == "" {
		return ""
	}

	if isError {
		return styles.ErrorText.Render(message)
	}
	return styles.SuccessText.Render(message)
}
