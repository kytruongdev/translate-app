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
	"translate-app/internal/repository"
)

//go:embed all:dist
var assets embed.FS

func main() {
	db, err := appdb.Open()
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	reg := repository.New(db)

	ctrls := controller.New(reg, &config.Keys)
	app, onStartup := handler.New(ctrls)

	err = wails.Run(&options.App{
		Title:  "translate-app",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        onStartup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
