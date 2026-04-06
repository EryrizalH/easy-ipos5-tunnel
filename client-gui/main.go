package main

import (
	"embed"
	"flag"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	startHidden := flag.Bool("hidden", false, "start window hidden")
	flag.Parse()

	app, err := NewApp(*startHidden)
	if err != nil {
		log.Fatal(err)
	}

	err = wails.Run(&options.App{
		Title:             "Easy Rathole Client",
		Width:             980,
		Height:            700,
		MinWidth:          860,
		MinHeight:         560,
		AssetServer:       &assetserver.Options{Assets: assets},
		OnStartup:         app.startup,
		OnDomReady:        app.domReady,
		OnBeforeClose:     app.beforeClose,
		OnShutdown:        app.shutdown,
		BackgroundColour:  &options.RGBA{R: 16, G: 22, B: 34, A: 1},
		StartHidden:       *startHidden,
		HideWindowOnClose: true,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
