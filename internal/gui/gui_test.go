package gui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuildCLIArgsIncludesSelectedOptions(t *testing.T) {
	args, err := BuildCLIArgs(Config{
		Emulator:     "pcsx2",
		CoverDir:     "/covers",
		GameListPath: "/cache/gamelist.cache",
		CoverType:    "3d",
		Fallback:     true,
		UseHTTP:      true,
		Workers:      8,
	})
	if err != nil {
		t.Fatalf("BuildCLIArgs returned error: %v", err)
	}

	want := []string{
		"-emulator", "pcsx2",
		"-covers", "/covers",
		"-gamelist", "/cache/gamelist.cache",
		"-type", "3d",
		"-workers", "8",
		"-fallback",
		"-http",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args=%#v want=%#v", args, want)
	}
}

func TestBuildCLIArgsRejectsMissingRequiredFields(t *testing.T) {
	_, err := BuildCLIArgs(Config{Emulator: "duckstation", CoverType: "default"})
	if err == nil {
		t.Fatal("BuildCLIArgs returned nil error")
	}
}

func TestNewAppResolvesRelativeCLIPath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	app := NewApp("dist/host/pscoverdl")
	want := filepath.Join(dir, "dist", "host", "pscoverdl")
	if app.cliPath != want {
		t.Fatalf("cliPath=%q want=%q", app.cliPath, want)
	}
}

func TestStartDownloadInvokesCLIWithArguments(t *testing.T) {
	if os.Getenv("PSCOVERDL_GUI_TEST_HELPER") == "1" {
		return
	}

	helperCLI := t.TempDir() + "/pscoverdl"
	if err := os.WriteFile(helperCLI, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	var gotPath string
	var gotArgs []string
	oldCommandContext := commandContext
	commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotPath = name
		gotArgs = append([]string(nil), args...)
		cmdArgs := []string{"-test.run=TestHelperProcess", "--"}
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cmdArgs...)
		cmd.Env = append(os.Environ(), "PSCOVERDL_GUI_TEST_HELPER=1")
		return cmd
	}
	t.Cleanup(func() {
		commandContext = oldCommandContext
	})

	app := NewApp(helperCLI)
	result := app.StartDownload(Config{
		Emulator:     "duckstation",
		CoverDir:     "/covers",
		GameListPath: "/cache/gamelist.cache",
		CoverType:    "default",
		Fallback:     true,
		Workers:      6,
	})

	if result.Error != "" {
		t.Fatalf("StartDownload error=%q output=%q", result.Error, result.Output)
	}
	if gotPath != helperCLI {
		t.Fatalf("path=%q", gotPath)
	}
	want := []string{"-emulator", "duckstation", "-covers", "/covers", "-gamelist", "/cache/gamelist.cache", "-type", "default", "-workers", "6", "-fallback"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("args=%#v want=%#v", gotArgs, want)
	}
}

func TestStartDownloadReportsMissingCLI(t *testing.T) {
	app := NewApp("/missing/pscoverdl")
	result := app.StartDownload(Config{
		Emulator:     "pcsx2",
		CoverDir:     "/covers",
		GameListPath: "/cache/gamelist.cache",
		CoverType:    "default",
		Workers:      4,
	})
	if !strings.Contains(result.Error, "pscoverdl CLI not found") {
		t.Fatalf("error=%q", result.Error)
	}
}

func TestDetectDefaultPathsPrefersExistingCache(t *testing.T) {
	home := t.TempDir()
	cache := home + "/Library/Application Support/PCSX2/cache"
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache+"/gamelist.cache", []byte("cache"), 0o600); err != nil {
		t.Fatal(err)
	}

	withPathDetection(t, "darwin", home, nil)
	cfg := defaultEmulatorConfig("pcsx2")

	wantCover := home + "/Library/Application Support/PCSX2/covers"
	wantCache := home + "/Library/Application Support/PCSX2/cache/gamelist.cache"
	if cfg.CoverDirectory != wantCover || cfg.GameCache != wantCache {
		t.Fatalf("cfg=%#v want cover=%q cache=%q", cfg, wantCover, wantCache)
	}
}

func TestDetectDefaultPathsIncludesLinuxFlatpakDuckStation(t *testing.T) {
	home := t.TempDir()
	covers := home + "/.var/app/org.duckstation.DuckStation/data/duckstation/covers"
	if err := os.MkdirAll(covers, 0o755); err != nil {
		t.Fatal(err)
	}

	withPathDetection(t, "linux", home, map[string]string{})
	cfg := defaultEmulatorConfig("duckstation")

	wantCache := home + "/.var/app/org.duckstation.DuckStation/data/duckstation/cache/gamelist.cache"
	if cfg.CoverDirectory != covers || cfg.GameCache != wantCache {
		t.Fatalf("cfg=%#v want cover=%q cache=%q", cfg, covers, wantCache)
	}
}

func TestLoadConfigFillsMissingDefaultPaths(t *testing.T) {
	home := t.TempDir()
	appData := home + "/appdata"
	duck := home + "/Documents/DuckStation"
	if err := os.MkdirAll(duck, 0o755); err != nil {
		t.Fatal(err)
	}
	withPathDetection(t, "windows", home, map[string]string{"APPDATA": appData})

	configDir := appData + "/pscoverdl"
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configDir+"/pscoverdl.json", []byte(`{"duckstation":{"useSSL":true},"pcsx2":{"coverDirectory":"custom","gameCache":"cache","useSSL":true}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := LoadConfigFile()
	if cfg.DuckStation.CoverDirectory != duck+"/covers" || cfg.DuckStation.GameCache != duck+"/cache/gamelist.cache" {
		t.Fatalf("duckstation=%#v", cfg.DuckStation)
	}
	if cfg.PCSX2.CoverDirectory != "custom" || cfg.PCSX2.GameCache != "cache" {
		t.Fatalf("pcsx2=%#v", cfg.PCSX2)
	}
}

func withPathDetection(t *testing.T, goos string, home string, env map[string]string) {
	t.Helper()
	oldOS := currentOS
	oldHome := userHomeDir
	oldGetenv := getenv
	currentOS = goos
	userHomeDir = func() (string, error) { return home, nil }
	getenv = func(key string) string {
		if env == nil {
			return ""
		}
		return env[key]
	}
	t.Cleanup(func() {
		currentOS = oldOS
		userHomeDir = oldHome
		getenv = oldGetenv
	})
}

func TestConfigForEmulatorMatchesLegacyCoverTypeAndSSL(t *testing.T) {
	cfg := ConfigForEmulator("pcsx2", EmulatorConfig{
		CoverDirectory: "/covers",
		GameCache:      "/cache/gamelist.cache",
		CoverType:      1,
		UseSSL:         false,
		Fallback:       true,
	})
	want := Config{Emulator: "pcsx2", CoverDir: "/covers", GameListPath: "/cache/gamelist.cache", CoverType: "3d", UseHTTP: true, Fallback: true, Workers: 4}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("cfg=%#v want=%#v", cfg, want)
	}
}

func TestHelperProcess(_ *testing.T) {
	if os.Getenv("PSCOVERDL_GUI_TEST_HELPER") != "1" {
		return
	}
	_, _ = os.Stdout.WriteString("download complete\n")
	os.Exit(0)
}
