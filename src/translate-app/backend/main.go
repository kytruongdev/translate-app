package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"translate-app/config"
	"translate-app/internal/controller"
	"translate-app/internal/handler"
	appdb "translate-app/internal/infra/db"
	"translate-app/internal/logger"
	"translate-app/internal/repository"
)

//go:embed all:dist
var assets embed.FS

func main() {
	appLog, err := logger.New()
	if err != nil {
		log.Fatalf("logger: %v", err)
	}
	defer appLog.Close()
	appLog.Info("AppStarted", "version", "1.0.0")

	db, err := appdb.Open()
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	reg := repository.New(db)

	ctrls := controller.New(reg, &config.Keys, appLog)
	app, onStartup := handler.New(ctrls)

	err = wails.Run(&options.App{
		Title:  "",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true, // Windows: prevent WebView2 from consuming drag events before Wails
		},
		OnStartup: onStartup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
