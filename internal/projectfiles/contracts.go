package projectfiles

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"seamgraph/internal/config"
	"seamgraph/internal/verification"
)

func LoadContracts(root string) (verification.Manifest, error) {
	m := verification.Manifest{Version: 1, Requirements: []verification.Requirement{}, Links: []verification.Link{}, Policies: map[string]string{}}
	b, err := os.ReadFile(filepath.Join(root, "seamgraph-contracts.yaml"))
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	var node yaml.Node
	d := yaml.NewDecoder(bytes.NewReader(b))
	if err = d.Decode(&node); err != nil {
		return m, err
	}
	if err = config.ValidateNodes(&node, map[*yaml.Node]bool{}); err != nil {
		return m, err
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return m, fmt.Errorf("contract manifest must contain one YAML document")
	}
	d = yaml.NewDecoder(bytes.NewReader(b))
	d.KnownFields(true)
	m = verification.Manifest{}
	err = d.Decode(&m)
	if err == nil && m.Version != 1 {
		err = fmt.Errorf("contract manifest requires version: 1")
	}
	return m, err
}
