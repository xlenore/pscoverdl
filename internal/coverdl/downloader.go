package coverdl

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// EmulatorPCSX2 identifies PCSX2 downloads.
	EmulatorPCSX2 = "pcsx2"
	// EmulatorDuckStation identifies DuckStation downloads.
	EmulatorDuckStation = "duckstation"

	// CoverTypeDefault selects flat JPG covers.
	CoverTypeDefault = "default"
	// CoverType3D selects 3D PNG covers.
	CoverType3D = "3d"

	ps1DefaultURL = "https://raw.githubusercontent.com/xlenore/psx-covers/main/covers/default"
	ps1ThreeDURL  = "https://raw.githubusercontent.com/xlenore/psx-covers/main/covers/3d"
	ps2DefaultURL = "https://raw.githubusercontent.com/xlenore/ps2-covers/main/covers/default"
	ps2ThreeDURL  = "https://raw.githubusercontent.com/xlenore/ps2-covers/main/covers/3d"
)

//go:embed resources/GameIndex.yaml resources/gamedb.json
var metadata embed.FS

var serialPattern = regexp.MustCompile(`\b[A-Z0-9]{4}-\d{5}\b`)

// Options configures a cover download run.
type Options struct {
	CoverDir       string
	GameListPath   string
	Emulator       string
	CoverType      string
	Fallback       bool
	UseSSL         bool
	Workers        int
	PCSX2NamesPath string
	DuckNamesPath  string
}

// CoverRequest describes primary and fallback URLs for one serial.
type CoverRequest struct {
	Serial             string
	URL                string
	OutputName         string
	FallbackURL        string
	FallbackOutputName string
}

// Result contains the outcome for one serial download.
type Result struct {
	Serial   string
	Name     string
	Path     string
	Fallback bool
	Err      error
}

// Summary contains aggregate results for a download run.
type Summary struct {
	Found      int
	Skipped    int
	Downloaded int
	Missing    int
	Failed     []Result
}

// ExistingCovers returns serials that already have JPG or PNG covers in dir.
func ExistingCovers(dir string) (map[string]struct{}, error) {
	covers := map[string]struct{}{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return covers, nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".jpg" || ext == ".png" {
			covers[strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))] = struct{}{}
		}
	}
	return covers, nil
}

// SerialsFromGameList extracts unique serials from a gamelist cache, excluding existing covers.
func SerialsFromGameList(path string, existing map[string]struct{}) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, serial := range serialPattern.FindAllString(strings.ToUpper(string(data)), -1) {
		if _, ok := existing[serial]; ok {
			continue
		}
		seen[serial] = struct{}{}
	}
	serials := make([]string, 0, len(seen))
	for serial := range seen {
		serials = append(serials, serial)
	}
	sort.Strings(serials)
	return serials, nil
}

// LoadDuckStationNames loads DuckStation serial-to-title metadata.
func LoadDuckStationNames(path string) (map[string]string, error) {
	data, err := readMetadata(path, "resources/gamedb.json")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Serial string `json:"serial"`
		Name   string `json:"name"`
	}
	unmarshalErr := json.Unmarshal(data, &rows)
	if unmarshalErr != nil {
		return nil, unmarshalErr
	}
	names := make(map[string]string, len(rows))
	for _, row := range rows {
		if row.Serial != "" {
			names[strings.ToUpper(row.Serial)] = row.Name
		}
	}
	return names, nil
}

// LoadPCSX2Names loads PCSX2 serial-to-title metadata.
func LoadPCSX2Names(path string) (map[string]string, error) {
	data, err := readMetadata(path, "resources/GameIndex.yaml")
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	var current string
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, "\r")
		if match := serialPattern.FindString(line); match != "" && !strings.HasPrefix(line, " ") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			current = strings.ToUpper(match)
			continue
		}
		trimmed := strings.TrimSpace(line)
		if current != "" && strings.HasPrefix(trimmed, "name:") {
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			names[current] = strings.Trim(name, `"'`)
		}
	}
	return names, nil
}

// BuildCoverRequests converts serials and options into cover download requests.
func BuildCoverRequests(serials []string, opts Options) ([]CoverRequest, error) {
	primary, fallback, primaryExt, fallbackExt, err := urlPlan(opts)
	if err != nil {
		return nil, err
	}
	reqs := make([]CoverRequest, 0, len(serials))
	for _, serial := range serials {
		req := CoverRequest{
			Serial:     strings.ToUpper(serial),
			URL:        primary + "/" + strings.ToUpper(serial) + primaryExt,
			OutputName: strings.ToUpper(serial) + primaryExt,
		}
		if opts.Fallback {
			req.FallbackURL = fallback + "/" + strings.ToUpper(serial) + fallbackExt
			req.FallbackOutputName = strings.ToUpper(serial) + fallbackExt
		}
		if !opts.UseSSL {
			req.URL = strings.Replace(req.URL, "https://", "http://", 1)
			req.FallbackURL = strings.Replace(req.FallbackURL, "https://", "http://", 1)
		}
		reqs = append(reqs, req)
	}
	return reqs, nil
}

// Run downloads covers according to opts and writes progress to out.
func Run(ctx context.Context, opts Options, out io.Writer) (Summary, error) {
	if opts.Workers <= 0 {
		opts.Workers = 4
	}
	if opts.CoverType == "" {
		opts.CoverType = CoverTypeDefault
	}
	if err := os.MkdirAll(opts.CoverDir, 0o750); err != nil {
		return Summary{}, err
	}
	existing, err := ExistingCovers(opts.CoverDir)
	if err != nil {
		return Summary{}, err
	}
	serials, err := SerialsFromGameList(opts.GameListPath, existing)
	if err != nil {
		return Summary{}, err
	}
	reqs, err := BuildCoverRequests(serials, opts)
	if err != nil {
		return Summary{}, err
	}
	names, err := loadNames(opts)
	if err != nil {
		return Summary{}, err
	}

	summary := Summary{Found: len(serials) + len(existing), Skipped: len(existing)}
	if len(reqs) == 0 {
		fmt.Fprintln(out, "[LOG]: All covers have already been downloaded")
		return summary, nil
	}
	fmt.Fprintf(out, "[LOG]: %d games queued\n", len(reqs))

	jobs := make(chan CoverRequest)
	results := make(chan Result)
	var wg sync.WaitGroup
	client := &http.Client{Timeout: 30 * time.Second}
	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for req := range jobs {
				results <- downloadOne(ctx, client, opts.CoverDir, req, names[req.Serial])
			}
		}()
	}
	go func() {
		for _, req := range reqs {
			jobs <- req
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	for result := range results {
		if result.Err != nil {
			summary.Missing++
			summary.Failed = append(summary.Failed, result)
			fmt.Fprintf(out, "[%s | %s] not found. Skipping...\n", result.Serial, displayName(result.Name))
			continue
		}
		summary.Downloaded++
		suffix := ""
		if result.Fallback {
			suffix = " (fallback)"
		}
		fmt.Fprintf(out, "%s | %s%s\n", result.Serial, displayName(result.Name), suffix)
	}
	return summary, nil
}

func urlPlan(opts Options) (primary, fallback, primaryExt, fallbackExt string, err error) {
	switch opts.Emulator {
	case EmulatorPCSX2:
		if opts.CoverType == CoverType3D {
			return ps2ThreeDURL, ps2DefaultURL, ".png", ".jpg", nil
		}
		if opts.CoverType == CoverTypeDefault || opts.CoverType == "" {
			return ps2DefaultURL, ps2ThreeDURL, ".jpg", ".png", nil
		}
	case EmulatorDuckStation:
		if opts.CoverType == CoverType3D {
			return ps1ThreeDURL, ps1DefaultURL, ".png", ".jpg", nil
		}
		if opts.CoverType == CoverTypeDefault || opts.CoverType == "" {
			return ps1DefaultURL, ps1ThreeDURL, ".jpg", ".png", nil
		}
	default:
		return "", "", "", "", fmt.Errorf("invalid emulator %q", opts.Emulator)
	}
	return "", "", "", "", fmt.Errorf("invalid cover type %q", opts.CoverType)
}

func readMetadata(path, embedded string) ([]byte, error) {
	if path != "" {
		return os.ReadFile(path)
	}
	return metadata.ReadFile(embedded)
}

func loadNames(opts Options) (map[string]string, error) {
	switch opts.Emulator {
	case EmulatorPCSX2:
		return LoadPCSX2Names(opts.PCSX2NamesPath)
	case EmulatorDuckStation:
		return LoadDuckStationNames(opts.DuckNamesPath)
	default:
		return nil, fmt.Errorf("invalid emulator %q", opts.Emulator)
	}
}

func downloadOne(ctx context.Context, client *http.Client, coverDir string, req CoverRequest, name string) Result {
	path := filepath.Join(coverDir, req.OutputName)
	if err := fetch(ctx, client, req.URL, path); err == nil {
		return Result{Serial: req.Serial, Name: name, Path: path}
	}
	if req.FallbackURL != "" {
		fallbackPath := filepath.Join(coverDir, req.FallbackOutputName)
		if err := fetch(ctx, client, req.FallbackURL, fallbackPath); err == nil {
			return Result{Serial: req.Serial, Name: name, Path: fallbackPath, Fallback: true}
		}
	}
	return Result{Serial: req.Serial, Name: name, Err: fmt.Errorf("not found")}
}

func fetch(ctx context.Context, client *http.Client, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return err
}

func displayName(name string) string {
	if name == "" {
		return "unknown"
	}
	return name
}
