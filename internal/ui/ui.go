// Package ui contains server-rendered components and locally bundled assets.
package ui

import (
	"embed"
	"encoding/json"
	"seamgraph/internal/application"
	"seamgraph/internal/verification"
	"slices"
)

//go:embed assets/*
var Assets embed.FS

func contains(ss []string, s string) bool { return slices.Contains(ss, s) }
func intersects(a, b []string) bool {
	for _, s := range a {
		if contains(b, s) {
			return true
		}
	}
	return false
}
func evidenceForSeam(e verification.Evidence, seam string, rs []verification.Requirement) bool {
	for _, r := range rs {
		if r.SeamID == seam && contains(e.RequirementIDs, r.ID) {
			return true
		}
	}
	return false
}

func comparisonJSON(result application.Result) string { b, _ := json.Marshal(result); return string(b) }
