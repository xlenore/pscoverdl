package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/xlenore/pscoverdl/internal/coverdl"
)

func main() {
	var opts coverdl.Options
	var useHTTP bool
	flag.StringVar(&opts.Emulator, "emulator", "", "emulator: pcsx2 or duckstation")
	flag.StringVar(&opts.CoverDir, "covers", "", "output cover directory")
	flag.StringVar(&opts.GameListPath, "gamelist", "", "path to gamelist.cache")
	flag.StringVar(&opts.CoverType, "type", coverdl.CoverTypeDefault, "cover type: default or 3d")
	flag.BoolVar(&opts.Fallback, "fallback", false, "try the other cover type when the selected type is missing")
	flag.BoolVar(&useHTTP, "http", false, "use HTTP instead of HTTPS for cover downloads")
	flag.IntVar(&opts.Workers, "workers", 4, "parallel download workers")
	flag.Parse()

	opts.Emulator = strings.ToLower(opts.Emulator)
	opts.CoverType = strings.ToLower(opts.CoverType)
	opts.UseSSL = !useHTTP

	if err := validate(opts); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR]:", err)
		flag.Usage()
		os.Exit(2)
	}

	if _, err := coverdl.Run(context.Background(), opts, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR]:", err)
		os.Exit(1)
	}
}

func validate(opts coverdl.Options) error {
	if opts.Emulator != coverdl.EmulatorPCSX2 && opts.Emulator != coverdl.EmulatorDuckStation {
		return fmt.Errorf("-emulator must be pcsx2 or duckstation")
	}
	if opts.CoverDir == "" {
		return fmt.Errorf("-covers is required")
	}
	if opts.GameListPath == "" {
		return fmt.Errorf("-gamelist is required")
	}
	if opts.CoverType != coverdl.CoverTypeDefault && opts.CoverType != coverdl.CoverType3D {
		return fmt.Errorf("-type must be default or 3d")
	}
	return nil
}
