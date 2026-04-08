package main

import (
	"embed"
	"flag"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	startHidden := flag.Bool("hidden", false, "start window hidden")
	flag.Parse()

	if err := run(*startHidden); err != nil {
		handleFatalStartupError(err)
	}
}

func run(startHidden bool) error {
	app, err := NewApp(startHidden)
	if err != nil {
		return err
	}

	err = wails.Run(&options.App{
		Title:             "Easy Rathole Client",
		Width:             420,
		Height:            560,
		MinWidth:          420,
		MinHeight:         560,
		DisableResize:     true,
		AssetServer:       &assetserver.Options{Assets: assets},
		OnStartup:         app.startup,
		OnDomReady:        app.domReady,
		OnBeforeClose:     app.beforeClose,
		OnShutdown:        app.shutdown,
		BackgroundColour:  &options.RGBA{R: 16, G: 22, B: 34, A: 1},
		StartHidden:       startHidden,
		HideWindowOnClose: true,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.easy-rathole.client-gui",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				app.onSecondInstanceLaunch()
			},
		},
		Bind: []interface{}{
			app,
		},
	})
	return err
}
