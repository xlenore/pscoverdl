package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/xlenore/pscoverdl/internal/gui"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cli := flag.String("cli", "", "path to pscoverdl CLI binary; defaults to pscoverdl next to this GUI binary")
	flag.Parse()

	app := gui.NewApp(*cli)
	if err := wails.Run(&options.App{
		Title:     "PSCoverDL - " + gui.CurrentVersion,
		Width:     640,
		Height:    460,
		MinWidth:  560,
		MinHeight: 420,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.Startup,
		Bind: []interface{}{
			app,
		},
	}); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR]:", err)
		os.Exit(1)
	}
}
