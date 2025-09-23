package main

import (
	"context"
	"log"
	"os"

	webassets "blackbox/frontend"
	"blackbox/internal/ui"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	app, err := ui.NewApp("./config/ui.json")
	if err != nil {
		log.Fatalf("init app: %v", err)
	}
	defer func() {
		if closeErr := app.Close(); closeErr != nil {
			log.Printf("error closing app: %v", closeErr)
		}
	}()

	err = wails.Run(&options.App{
		Title:            "Blackbox",
		Width:            1024,
		Height:           720,
		AssetServer:      &assetserver.Options{Assets: webassets.Assets},
		Logger:           logger.NewDefaultLogger(),
		BackgroundColour: &options.RGBA{R: 20, G: 20, B: 20, A: 1},
		OnStartup:        func(ctx context.Context) { app.SetUIContext(ctx) },
		Bind:             []interface{}{app},
	})
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
}
