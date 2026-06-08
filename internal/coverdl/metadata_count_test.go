package coverdl

import "testing"

func TestEmbeddedMetadataLoadsRealDatabases(t *testing.T) {
	ps2, err := LoadPCSX2Names("")
	if err != nil {
		t.Fatal(err)
	}
	ps1, err := LoadDuckStationNames("")
	if err != nil {
		t.Fatal(err)
	}
	if len(ps2) < 1000 || len(ps1) < 1000 {
		t.Fatalf("metadata counts too low: ps2=%d ps1=%d", len(ps2), len(ps1))
	}
}
