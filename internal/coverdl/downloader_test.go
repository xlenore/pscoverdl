package coverdl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSerialsFromGameListAreUniqueAndSkipExistingCovers(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "gamelist.cache")
	contents := "Alpha SLUS-20312 duplicate SLUS-20312 beta SCES-50003 ignored ABC-12345"
	if err := os.WriteFile(cache, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	serials, err := SerialsFromGameList(cache, map[string]struct{}{"SLUS-20312": {}})
	if err != nil {
		t.Fatalf("SerialsFromGameList returned error: %v", err)
	}

	want := []string{"SCES-50003"}
	if len(serials) != len(want) || serials[0] != want[0] {
		t.Fatalf("serials = %#v, want %#v", serials, want)
	}
}

func TestExistingCoversFindsJpgAndPngStems(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"SLUS-20312.jpg", "SCES-50003.png", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	covers, err := ExistingCovers(dir)
	if err != nil {
		t.Fatalf("ExistingCovers returned error: %v", err)
	}

	for _, serial := range []string{"SLUS-20312", "SCES-50003"} {
		if _, ok := covers[serial]; !ok {
			t.Fatalf("missing existing cover %s in %#v", serial, covers)
		}
	}
	if _, ok := covers["notes"]; ok {
		t.Fatalf("non-cover file was included: %#v", covers)
	}
}

func TestBuildCoverRequestsUsesEmulatorAndFallback(t *testing.T) {
	reqs, err := BuildCoverRequests([]string{"SLUS-20312"}, Options{
		Emulator:  EmulatorPCSX2,
		CoverType: CoverType3D,
		Fallback:  true,
		UseSSL:    true,
	})
	if err != nil {
		t.Fatalf("BuildCoverRequests returned error: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("request count = %d, want 1", len(reqs))
	}

	got := reqs[0]
	if got.Serial != "SLUS-20312" {
		t.Fatalf("serial = %q", got.Serial)
	}
	if got.URL != "https://raw.githubusercontent.com/xlenore/ps2-covers/main/covers/3d/SLUS-20312.png" {
		t.Fatalf("url = %q", got.URL)
	}
	if got.OutputName != "SLUS-20312.png" {
		t.Fatalf("output = %q", got.OutputName)
	}
	if got.FallbackURL != "https://raw.githubusercontent.com/xlenore/ps2-covers/main/covers/default/SLUS-20312.jpg" {
		t.Fatalf("fallback url = %q", got.FallbackURL)
	}
	if got.FallbackOutputName != "SLUS-20312.jpg" {
		t.Fatalf("fallback output = %q", got.FallbackOutputName)
	}
}

func TestLoadDuckStationNamesFromJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gamedb.json")
	if err := os.WriteFile(path, []byte(`[{"serial":"SLUS-20312","name":"Game Name"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	names, err := LoadDuckStationNames(path)
	if err != nil {
		t.Fatalf("LoadDuckStationNames returned error: %v", err)
	}
	if names["SLUS-20312"] != "Game Name" {
		t.Fatalf("name = %q", names["SLUS-20312"])
	}
}

func TestLoadPCSX2NamesFromGameIndexYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "GameIndex.yaml")
	yaml := "SLUS-20312:\n  name: Example Game\nSCES-50003:\n  region: EU\n  name: Another Game\n"
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	names, err := LoadPCSX2Names(path)
	if err != nil {
		t.Fatalf("LoadPCSX2Names returned error: %v", err)
	}
	if names["SLUS-20312"] != "Example Game" || names["SCES-50003"] != "Another Game" {
		t.Fatalf("names = %#v", names)
	}
}

func TestDownloadOneWritesPrimaryCover(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("cover"))
	}))
	defer server.Close()

	dir := t.TempDir()
	result := downloadOne(context.Background(), server.Client(), dir, CoverRequest{
		Serial:     "SLUS-20312",
		URL:        server.URL + "/SLUS-20312.jpg",
		OutputName: "SLUS-20312.jpg",
	}, "Game")

	if result.Err != nil {
		t.Fatalf("downloadOne returned error: %v", result.Err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "SLUS-20312.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "cover" || result.Fallback {
		t.Fatalf("data=%q fallback=%v", data, result.Fallback)
	}
}

func TestDownloadOneUsesFallbackWhenPrimaryMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fallback.png" {
			_, _ = w.Write([]byte("fallback"))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	dir := t.TempDir()
	result := downloadOne(context.Background(), server.Client(), dir, CoverRequest{
		Serial:             "SLUS-20312",
		URL:                server.URL + "/missing.jpg",
		OutputName:         "SLUS-20312.jpg",
		FallbackURL:        server.URL + "/fallback.png",
		FallbackOutputName: "SLUS-20312.png",
	}, "Game")

	if result.Err != nil {
		t.Fatalf("downloadOne returned error: %v", result.Err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "SLUS-20312.png"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "fallback" || !result.Fallback {
		t.Fatalf("data=%q fallback=%v", data, result.Fallback)
	}
}
