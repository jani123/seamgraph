package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStrictContractManifest(t *testing.T) {
	root := t.TempDir()
	m, err := LoadContracts(root)
	if err != nil || m.Version != 1 {
		t.Fatal("missing optional manifest not report-only", m, err)
	}
	for _, data := range []string{"links: []\n", "version: 2\n", "version: 1\nversion: 1\n", "version: 1\nunknown: true\n", "version: 1\n---\nversion: 1\n", "version: 1\nlinks: null\n"} {
		if err := os.WriteFile(filepath.Join(root, "seamgraph-contracts.yaml"), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadContracts(root); err == nil {
			t.Fatal("invalid contract manifest accepted", data)
		}
	}
}
