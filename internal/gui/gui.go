/*
⣞⢽⢪⢣⢣⢣⢫⡺⡵⣝⡮⣗⢷⢽⢽⢽⣮⡷⡽⣜⣜⢮⢺⣜⢷⢽⢝⡽⣝
⠸⡸⠜⠕⠕⠁⢁⢇⢏⢽⢺⣪⡳⡝⣎⣏⢯⢞⡿⣟⣷⣳⢯⡷⣽⢽⢯⣳⣫⠇
⠀⠀⢀⢀⢄⢬⢪⡪⡎⣆⡈⠚⠜⠕⠇⠗⠝⢕⢯⢫⣞⣯⣿⣻⡽⣏⢗⣗⠏⠀
⠀⠪⡪⡪⣪⢪⢺⢸⢢⢓⢆⢤⢀⠀⠀⠀⠀⠈⢊⢞⡾⣿⡯⣏⢮⠷⠁⠀⠀
⠀⠀⠀⠈⠊⠆⡃⠕⢕⢇⢇⢇⢇⢇⢏⢎⢎⢆⢄⠀⢑⣽⣿⢝⠲⠉⠀⠀⠀⠀
⠀⠀⠀⠀⠀⡿⠂⠠⠀⡇⢇⠕⢈⣀⠀⠁⠡⠣⡣⡫⣂⣿⠯⢪⠰⠂⠀⠀⠀⠀
⠀⠀⠀⠀⡦⡙⡂⢀⢤⢣⠣⡈⣾⡃⠠⠄⠀⡄⢱⣌⣶⢏⢊⠂⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⢝⡲⣜⡮⡏⢎⢌⢂⠙⠢⠐⢀⢘⢵⣽⣿⡿⠁⠁⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠨⣺⡺⡕⡕⡱⡑⡆⡕⡅⡕⡜⡼⢽⡻⠏⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⣼⣳⣫⣾⣵⣗⡵⡱⡡⢣⢑⢕⢜⢕⡝⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⣴⣿⣾⣿⣿⣿⡿⡽⡑⢌⠪⡢⡣⣣⡟⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⡟⡾⣿⢿⢿⢵⣽⣾⣼⣘⢸⢸⣞⡟⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠁⠇⠡⠩⡫⢿⣝⡻⡮⣒⢽⠋⠀⠀⠀

    NO COVERS?
*/

package gui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// CurrentVersion is the GUI version displayed in the window title and update check.
const CurrentVersion = "1.1"

// Config contains CLI options collected by the GUI.
type Config struct {
	CLIPath      string `json:"cliPath"`
	Emulator     string `json:"emulator"`
	CoverDir     string `json:"coverDir"`
	GameListPath string `json:"gameListPath"`
	CoverType    string `json:"coverType"`
	Fallback     bool   `json:"fallback"`
	UseHTTP      bool   `json:"useHTTP"`
	Workers      int    `json:"workers"`
}

// EmulatorConfig stores persisted form values for one emulator tab.
type EmulatorConfig struct {
	CoverDirectory string `json:"coverDirectory"`
	GameCache      string `json:"gameCache"`
	CoverType      int    `json:"coverType"`
	UseSSL         bool   `json:"useSSL"`
	Fallback       bool   `json:"fallback"`
}

// StoredConfig stores persisted GUI settings for both emulator tabs.
type StoredConfig struct {
	DuckStation EmulatorConfig `json:"duckstation"`
	PCSX2       EmulatorConfig `json:"pcsx2"`
}

// DownloadResult is returned to the Wails frontend after invoking the CLI.
type DownloadResult struct {
	Command string `json:"command"`
	Output  string `json:"output"`
	Error   string `json:"error"`
}

// UpdateStatus describes the remote version check result.
type UpdateStatus struct {
	Version             string `json:"version"`
	LatestVersion       string `json:"latestVersion"`
	NewVersionAvailable bool   `json:"newVersionAvailable"`
}

// App exposes Wails-bound GUI methods.
type App struct {
	ctx     context.Context
	cliPath string
}

var commandContext = exec.CommandContext
var statFile = os.Stat
var getenv = os.Getenv
var userHomeDir = os.UserHomeDir
var currentOS = runtime.GOOS

// NewApp creates a Wails GUI backend.
func NewApp(cliPath string) *App {
	return &App{cliPath: normalizeCLIPath(cliPath)}
}

func normalizeCLIPath(cliPath string) string {
	cliPath = strings.TrimSpace(cliPath)
	if cliPath == "" || filepath.IsAbs(cliPath) {
		return cliPath
	}
	absPath, err := filepath.Abs(cliPath)
	if err != nil {
		return cliPath
	}
	return absPath
}

// Startup stores the Wails context for dialogs and cancellation.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// BuildCLIArgs converts GUI config into pscoverdl CLI arguments.
func BuildCLIArgs(cfg Config) ([]string, error) {
	cfg.Emulator = strings.ToLower(strings.TrimSpace(cfg.Emulator))
	cfg.CoverType = strings.ToLower(strings.TrimSpace(cfg.CoverType))
	if cfg.CoverType == "" {
		cfg.CoverType = "default"
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.Emulator != "pcsx2" && cfg.Emulator != "duckstation" {
		return nil, errors.New("emulator must be pcsx2 or duckstation")
	}
	if strings.TrimSpace(cfg.CoverDir) == "" {
		return nil, errors.New("cover directory is required")
	}
	if strings.TrimSpace(cfg.GameListPath) == "" {
		return nil, errors.New("gamelist.cache file is required")
	}
	if cfg.CoverType != "default" && cfg.CoverType != "3d" {
		return nil, errors.New("cover type must be default or 3d")
	}

	args := []string{
		"-emulator", cfg.Emulator,
		"-covers", cfg.CoverDir,
		"-gamelist", cfg.GameListPath,
		"-type", cfg.CoverType,
		"-workers", strconv.Itoa(cfg.Workers),
	}
	if cfg.Fallback {
		args = append(args, "-fallback")
	}
	if cfg.UseHTTP {
		args = append(args, "-http")
	}
	return args, nil
}

// DefaultCLIPath returns the CLI binary path expected next to the GUI binary.
func DefaultCLIPath() string {
	exe, err := os.Executable()
	if err != nil {
		return executableName()
	}
	return filepath.Join(filepath.Dir(exe), executableName())
}

// ConfigPath returns the platform-specific GUI configuration path.
func ConfigPath() (string, error) {
	var base string
	switch currentOS {
	case "windows":
		base = getenv("APPDATA")
		if base == "" {
			base = homeDir()
		}
	case "darwin":
		base = filepath.Join(homeDir(), "Library", "Application Support")
	default:
		base = getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = filepath.Join(homeDir(), ".config")
		}
	}
	if base == "" {
		return "", errors.New("cannot determine config directory")
	}
	dir := filepath.Join(base, "pscoverdl")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "pscoverdl.json"), nil
}

// LoadConfig loads persisted GUI settings for the frontend.
func (a *App) LoadConfig() StoredConfig {
	return LoadConfigFile()
}

// LoadConfigFile loads persisted GUI settings from disk.
func LoadConfigFile() StoredConfig {
	cfg := defaultStoredConfig()
	path, err := ConfigPath()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	applyMissingDefaults(&cfg)
	return cfg
}

// DetectDefaults returns detected paths for an emulator tab.
func (a *App) DetectDefaults(emulator string) EmulatorConfig {
	return defaultEmulatorConfig(emulator)
}

// SaveConfig persists GUI settings from the frontend.
func (a *App) SaveConfig(cfg StoredConfig) string {
	if err := SaveConfigFile(cfg); err != nil {
		return err.Error()
	}
	return ""
}

// SaveConfigFile writes GUI settings to disk.
func SaveConfigFile(cfg StoredConfig) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// SelectCoverDirectory opens a native directory picker.
func (a *App) SelectCoverDirectory() string {
	if a.ctx == nil {
		return ""
	}
	path, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{Title: "Select cover directory"})
	if err != nil {
		return ""
	}
	return path
}

// SelectGameCache opens a native gamelist.cache picker.
func (a *App) SelectGameCache() string {
	if a.ctx == nil {
		return ""
	}
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title:   "Select gamelist.cache file",
		Filters: []wailsRuntime.FileFilter{{DisplayName: "Game cache (*.cache)", Pattern: "*.cache"}},
	})
	if err != nil {
		return ""
	}
	return path
}

// StartDownload invokes the CLI with options selected in the GUI.
func (a *App) StartDownload(cfg Config) DownloadResult {
	args, err := BuildCLIArgs(cfg)
	if err != nil {
		return DownloadResult{Error: err.Error()}
	}
	cliPath := strings.TrimSpace(cfg.CLIPath)
	if cliPath == "" {
		cliPath = strings.TrimSpace(a.cliPath)
	}
	if cliPath == "" {
		cliPath = DefaultCLIPath()
	}

	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	if _, statErr := statFile(cliPath); statErr != nil {
		return DownloadResult{Error: fmt.Sprintf("pscoverdl CLI not found at %s. Run `just build-cli` or pass `-cli /path/to/pscoverdl`.", cliPath)}
	}

	cmd := commandContext(ctx, cliPath, args...)
	var output bytes.Buffer
	var outputMu sync.Mutex
	appendOutput := func(line string) {
		outputMu.Lock()
		defer outputMu.Unlock()
		output.WriteString(line)
		output.WriteByte('\n')
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "download-progress", line)
		}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return DownloadResult{Error: err.Error()}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return DownloadResult{Error: err.Error()}
	}

	commandLine := cliPath + " " + strings.Join(args, " ")
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "download-progress", commandLine)
	}
	if err = cmd.Start(); err != nil {
		return DownloadResult{Command: commandLine, Error: err.Error()}
	}

	var wg sync.WaitGroup
	for _, pipe := range []interface{ Read([]byte) (int, error) }{stdout, stderr} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(pipe)
			for scanner.Scan() {
				appendOutput(scanner.Text())
			}
		}()
	}
	err = cmd.Wait()
	wg.Wait()
	outputMu.Lock()
	result := DownloadResult{Command: commandLine, Output: output.String()}
	outputMu.Unlock()
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

// CheckUpdates checks GitHub for a newer version.
func (a *App) CheckUpdates() UpdateStatus {
	status := UpdateStatus{Version: CurrentVersion, LatestVersion: CurrentVersion}
	client := http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://github.com/xlenore/pscoverdl/raw/main/VERSION", nil)
	if err != nil {
		return status
	}
	resp, err := client.Do(req)
	if err != nil {
		return status
	}
	defer resp.Body.Close()
	buf := make([]byte, 32)
	n, _ := resp.Body.Read(buf)
	latest := strings.TrimSpace(string(buf[:n]))
	if latest != "" {
		status.LatestVersion = latest
		status.NewVersionAvailable = latest != CurrentVersion
	}
	return status
}

func defaultStoredConfig() StoredConfig {
	return StoredConfig{
		DuckStation: defaultEmulatorConfig("duckstation"),
		PCSX2:       defaultEmulatorConfig("pcsx2"),
	}
}

func defaultEmulatorConfig(emulator string) EmulatorConfig {
	coverDir, gameCache := detectDefaultPaths(emulator)
	return EmulatorConfig{CoverDirectory: coverDir, GameCache: gameCache, CoverType: 0, UseSSL: true}
}

func applyMissingDefaults(cfg *StoredConfig) {
	fillMissingDefaults(&cfg.DuckStation, "duckstation")
	fillMissingDefaults(&cfg.PCSX2, "pcsx2")
}

func fillMissingDefaults(cfg *EmulatorConfig, emulator string) {
	defaults := defaultEmulatorConfig(emulator)
	if strings.TrimSpace(cfg.CoverDirectory) == "" {
		cfg.CoverDirectory = defaults.CoverDirectory
	}
	if strings.TrimSpace(cfg.GameCache) == "" {
		cfg.GameCache = defaults.GameCache
	}
}

type defaultPathSet struct {
	Root   string
	Covers string
	Cache  string
}

func detectDefaultPaths(emulator string) (string, string) {
	candidates := defaultPathCandidates(strings.ToLower(strings.TrimSpace(emulator)))
	for _, candidate := range candidates {
		if isFile(candidate.Cache) {
			return candidate.Covers, candidate.Cache
		}
	}
	for _, candidate := range candidates {
		if isDir(candidate.Covers) {
			return candidate.Covers, candidate.Cache
		}
	}
	for _, candidate := range candidates {
		if isDir(candidate.Root) {
			return candidate.Covers, candidate.Cache
		}
	}
	return "", ""
}

func defaultPathCandidates(emulator string) []defaultPathSet {
	switch emulator {
	case "pcsx2":
		return defaultPathCandidatesFor("PCSX2", "pcsx2", "net.pcsx2.PCSX2")
	case "duckstation":
		return defaultPathCandidatesFor("DuckStation", "duckstation", "org.duckstation.DuckStation")
	default:
		return nil
	}
}

func defaultPathCandidatesFor(name, lowerName, flatpakID string) []defaultPathSet {
	home := homeDir()
	var roots []string
	switch currentOS {
	case "windows":
		roots = appendNonEmpty(roots,
			filepath.Join(getenv("ProgramFiles"), name),
			filepath.Join(getenv("ProgramFiles(x86)"), name),
			filepath.Join(home, "Documents", name),
			filepath.Join(getenv("APPDATA"), name),
		)
	case "darwin":
		roots = appendNonEmpty(roots, filepath.Join(home, "Library", "Application Support", name))
	default:
		configHome := getenv("XDG_CONFIG_HOME")
		if configHome == "" && home != "" {
			configHome = filepath.Join(home, ".config")
		}
		dataHome := getenv("XDG_DATA_HOME")
		if dataHome == "" && home != "" {
			dataHome = filepath.Join(home, ".local", "share")
		}
		roots = appendNonEmpty(roots,
			filepath.Join(configHome, lowerName),
			filepath.Join(configHome, name),
			filepath.Join(dataHome, lowerName),
			filepath.Join(dataHome, name),
			filepath.Join(home, ".var", "app", flatpakID, "config", lowerName),
			filepath.Join(home, ".var", "app", flatpakID, "config", name),
			filepath.Join(home, ".var", "app", flatpakID, "data", lowerName),
			filepath.Join(home, ".var", "app", flatpakID, "data", name),
		)
	}

	sets := make([]defaultPathSet, 0, len(roots))
	for _, root := range roots {
		sets = append(sets, defaultPathSet{
			Root:   root,
			Covers: filepath.Join(root, "covers"),
			Cache:  filepath.Join(root, "cache", "gamelist.cache"),
		})
	}
	return sets
}

func appendNonEmpty(values []string, paths ...string) []string {
	for _, path := range paths {
		if path != "" && path != "." {
			values = append(values, path)
		}
	}
	return values
}

func isDir(path string) bool {
	info, err := statFile(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := statFile(path)
	return err == nil && !info.IsDir()
}

func homeDir() string {
	home, _ := userHomeDir()
	return home
}

func executableName() string {
	if runtime.GOOS == "windows" {
		return "pscoverdl.exe"
	}
	return "pscoverdl"
}

// CoverTypeString converts the legacy GUI cover type integer to a CLI value.
func CoverTypeString(value int) string {
	if value == 1 {
		return "3d"
	}
	return "default"
}

// ConfigForEmulator converts persisted emulator settings to CLI config.
func ConfigForEmulator(emulator string, cfg EmulatorConfig) Config {
	return Config{
		Emulator:     emulator,
		CoverDir:     cfg.CoverDirectory,
		GameListPath: cfg.GameCache,
		CoverType:    CoverTypeString(cfg.CoverType),
		UseHTTP:      !cfg.UseSSL,
		Fallback:     cfg.Fallback,
		Workers:      4,
	}
}

func (r DownloadResult) String() string {
	if r.Error != "" {
		return fmt.Sprintf("%s\n%s", r.Error, r.Output)
	}
	return r.Output
}
