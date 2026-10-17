// Package runtimecheck reports the external tools used by the adapters.
package runtimecheck

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Tool struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}
type spec struct {
	name, purpose string
	args          []string
	major, minor  int
}

var tools = []spec{
	{"go", "Go analysis", []string{"version"}, 1, 25},
	{"node", "TypeScript analysis (compiler embedded)", []string{"--version"}, 18, 0},
	{"python3", "Python AST analysis", []string{"--version"}, 3, 10},
	{"git", "Repository access and revision comparison", []string{"--version"}, 0, 0},
	{"ssh", "SSH repository access", []string{"-V"}, 0, 0},
}
var version = regexp.MustCompile(`(?:go|v|Python )([0-9]+)\.([0-9]+)`)

func check(ctx context.Context, s spec) Tool {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	t := Tool{Name: s.name, Purpose: s.purpose}
	b, err := exec.CommandContext(ctx, s.name, s.args...).CombinedOutput()
	t.Version = strings.TrimSpace(string(b))
	if err != nil {
		t.Error = fmt.Sprintf("%s unavailable: %v", s.name, err)
		return t
	}
	if s.major != 0 {
		m := version.FindStringSubmatch(t.Version)
		if len(m) != 3 {
			t.Error = "could not determine runtime version"
			return t
		}
		major, _ := strconv.Atoi(m[1])
		minor, _ := strconv.Atoi(m[2])
		if major < s.major || major == s.major && minor < s.minor {
			t.Error = fmt.Sprintf("requires %s %d.%d or newer", s.name, s.major, s.minor)
		}
	}
	return t
}
func Check(ctx context.Context, names ...string) []Tool {
	result := make([]Tool, 0, len(tools))
	for _, s := range tools {
		selected := len(names) == 0
		for _, name := range names {
			if name == s.name {
				selected = true
			}
		}
		if !selected {
			continue
		}
		result = append(result, check(ctx, s))
	}
	return result
}

// ToolsForLanguages selects only requested runtimes. Git and SSH are explicit
// selections because static analysis of a local project need not fetch a repo.
func ToolsForLanguages(languages string) ([]string, error) {
	names := []string{}
	seen := map[string]bool{}
	for _, language := range strings.Split(languages, ",") {
		var name string
		switch strings.TrimSpace(language) {
		case "go":
			name = "go"
		case "typescript":
			name = "node"
		case "python":
			name = "python3"
		case "git":
			name = "git"
		case "ssh":
			name = "ssh"
		default:
			return nil, fmt.Errorf("unknown runtime selection %q; use go,typescript,python,git,ssh", language)
		}
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	return names, nil
}
