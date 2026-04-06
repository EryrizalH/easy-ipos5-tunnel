package main

import (
	"context"
	_ "embed"
	"fmt"
	"sync"
	"time"

	"easy-rathole/client-gui/internal/appcore"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed icon.ico
var trayIcon []byte

type App struct {
	ctx         context.Context
	monitor     *appcore.Monitor
	cfgStore    *appcore.ConfigStore
	cfg         appcore.AppConfig
	cfgMu       sync.RWMutex
	startHidden bool
}

func NewApp(startHidden bool) (*App, error) {
	cfgStore, err := appcore.NewConfigStore()
	if err != nil {
		return nil, err
	}
	cfg, err := cfgStore.Load()
	if err != nil {
		return nil, err
	}

	return &App{
		monitor:     appcore.NewMonitor("EasyRatholeClient"),
		cfgStore:    cfgStore,
		cfg:         cfg,
		startHidden: startHidden,
	}, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go systray.Run(a.onTrayReady, func() {})
}

func (a *App) domReady(ctx context.Context) {
	if a.startHidden {
		runtime.WindowHide(ctx)
		return
	}
	runtime.WindowShow(ctx)
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	runtime.WindowHide(ctx)
	return true
}

func (a *App) shutdown(ctx context.Context) {
	systray.Quit()
}

func (a *App) onTrayReady() {
	systray.SetTitle("Easy Rathole")
	systray.SetTooltip("Easy Rathole Client GUI")
	if len(trayIcon) > 0 {
		systray.SetIcon(trayIcon)
	}

	openItem := systray.AddMenuItem("Open Dashboard", "Tampilkan dashboard")
	refreshItem := systray.AddMenuItem("Refresh", "Refresh status")
	systray.AddSeparator()
	startItem := systray.AddMenuItem("Start Service", "Start EasyRatholeClient")
	stopItem := systray.AddMenuItem("Stop Service", "Stop EasyRatholeClient")
	restartItem := systray.AddMenuItem("Restart Service", "Restart EasyRatholeClient")
	systray.AddSeparator()
	exitItem := systray.AddMenuItem("Exit", "Keluar aplikasi")

	go func() {
		for {
			select {
			case <-openItem.ClickedCh:
				if a.ctx != nil {
					runtime.WindowUnminimise(a.ctx)
					runtime.WindowShow(a.ctx)
				}
			case <-refreshItem.ClickedCh:
				_ = a.GetStatus()
			case <-startItem.ClickedCh:
				_ = a.StartService()
			case <-stopItem.ClickedCh:
				_ = a.StopService()
			case <-restartItem.ClickedCh:
				_ = a.RestartService()
			case <-exitItem.ClickedCh:
				if a.ctx != nil {
					runtime.Quit(a.ctx)
				}
				return
			}
		}
	}()
}

func (a *App) GetStatus() appcore.StatusSnapshot {
	cfg := a.getConfig()
	snapshot := a.monitor.Snapshot(cfg.ConfigPath)
	if snapshot.LastCheckedAt == "" {
		snapshot.LastCheckedAt = time.Now().Format(time.RFC3339)
	}
	return snapshot
}

func (a *App) StartService() appcore.ActionResult {
	if err := appcore.StartService("EasyRatholeClient"); err != nil {
		return appcore.ActionResult{OK: false, Message: err.Error()}
	}
	return appcore.ActionResult{OK: true, Message: "Service berhasil dijalankan"}
}

func (a *App) StopService() appcore.ActionResult {
	if err := appcore.StopService("EasyRatholeClient"); err != nil {
		return appcore.ActionResult{OK: false, Message: err.Error()}
	}
	return appcore.ActionResult{OK: true, Message: "Service berhasil dihentikan"}
}

func (a *App) RestartService() appcore.ActionResult {
	if err := appcore.RestartService("EasyRatholeClient"); err != nil {
		return appcore.ActionResult{OK: false, Message: err.Error()}
	}
	return appcore.ActionResult{OK: true, Message: "Service berhasil direstart"}
}

func (a *App) SetConfigPath(path string) appcore.ActionResult {
	cfg := a.getConfig()
	cfg.ConfigPath = path
	if err := a.saveConfig(cfg); err != nil {
		return appcore.ActionResult{OK: false, Message: err.Error()}
	}
	return appcore.ActionResult{OK: true, Message: "Path client.toml berhasil disimpan"}
}

func (a *App) EnableAutoStart() appcore.ActionResult {
	if err := appcore.EnableTaskSchedulerAutoStart(); err != nil {
		return appcore.ActionResult{
			OK:      false,
			Message: fmt.Sprintf("%s. Coba jalankan aplikasi sebagai Administrator lalu ulangi.", err.Error()),
		}
	}
	cfg := a.getConfig()
	cfg.AutoStartEnabled = true
	if err := a.saveConfig(cfg); err != nil {
		return appcore.ActionResult{OK: false, Message: err.Error()}
	}
	return appcore.ActionResult{OK: true, Message: "Auto-start berhasil diaktifkan"}
}

func (a *App) DisableAutoStart() appcore.ActionResult {
	if err := appcore.DisableTaskSchedulerAutoStart(); err != nil {
		return appcore.ActionResult{OK: false, Message: err.Error()}
	}
	cfg := a.getConfig()
	cfg.AutoStartEnabled = false
	if err := a.saveConfig(cfg); err != nil {
		return appcore.ActionResult{OK: false, Message: err.Error()}
	}
	return appcore.ActionResult{OK: true, Message: "Auto-start berhasil dinonaktifkan"}
}

func (a *App) GetConfig() appcore.AppConfig {
	return a.getConfig()
}

func (a *App) getConfig() appcore.AppConfig {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg
}

func (a *App) saveConfig(cfg appcore.AppConfig) error {
	if err := a.cfgStore.Save(cfg); err != nil {
		return err
	}
	a.cfgMu.Lock()
	a.cfg = cfg
	a.cfgMu.Unlock()
	return nil
}
