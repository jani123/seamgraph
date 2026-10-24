package projectfiles

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed agentassets/rules.md
var agentRules string

//go:embed agentassets/seamgraph/SKILL.md
var agentSkill string

const (
	agentBegin = "<!-- seamgraph:begin -->"
	agentEnd   = "<!-- seamgraph:end -->"
	skillOwner = "<!-- seamgraph:managed-skill -->"
	skillPath  = ".agents/skills/seamgraph/SKILL.md"
)

type agentFile struct {
	path          string
	before, after []byte
	mode          os.FileMode
}

// SetupAgent installs managed guidance without changing other project instructions.
// Both destinations are validated before writing; unchanged files are not rewritten.
func SetupAgent(root string) ([]string, error) {
	scope, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer func() { _ = scope.Close() }()
	command := "seamgraph"
	for _, name := range []string{"devbox.json", "devbox.jsonc"} {
		if _, err := scope.Stat(name); err == nil {
			command = "devbox run seamgraph"
			break
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	render := func(s string) string { return strings.ReplaceAll(s, "{{command}}", command) }
	files, err := prepareAgentFiles(scope, render(agentRules), render(agentSkill))
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, file := range files {
		if bytes.Equal(file.before, file.after) {
			continue
		}
		if err := writeAgentFile(scope, file); err != nil {
			return changed, fmt.Errorf("write %s: %w", file.path, err)
		}
		changed = append(changed, file.path)
	}
	return changed, nil
}

func prepareAgentFiles(scope *os.Root, rules, skill string) ([]agentFile, error) {
	instructions, err := readAgentFile(scope, "AGENTS.md")
	if err != nil {
		return nil, err
	}
	instructions.after, err = mergeAgentRules(instructions.before, rules)
	if err != nil {
		return nil, err
	}
	target, err := readAgentFile(scope, skillPath)
	if err != nil {
		return nil, err
	}
	if target.before != nil && !strings.Contains(string(target.before), skillOwner) {
		return nil, fmt.Errorf("%s already exists without SeamGraph's ownership marker; move or rename it before setup", skillPath)
	}
	target.after = []byte(skill)
	return []agentFile{instructions, target}, nil
}

func readAgentFile(scope *os.Root, name string) (agentFile, error) {
	file := agentFile{path: name, mode: 0644}
	info, err := scope.Lstat(name)
	if os.IsNotExist(err) {
		return file, nil
	}
	if err != nil {
		return file, err
	}
	if !info.Mode().IsRegular() {
		return file, fmt.Errorf("%s must be a regular file, not a symlink or directory", name)
	}
	file.mode = info.Mode().Perm()
	file.before, err = scope.ReadFile(name)
	return file, err
}

func mergeAgentRules(before []byte, block string) ([]byte, error) {
	text := string(before)
	begins, ends := strings.Count(text, agentBegin), strings.Count(text, agentEnd)
	if begins == 0 && ends == 0 {
		separator := ""
		if len(text) > 0 {
			separator = "\n\n"
			if strings.HasSuffix(text, "\n") {
				separator = "\n"
			}
		}
		return []byte(text + separator + block), nil
	}
	start, end := strings.Index(text, agentBegin), strings.Index(text, agentEnd)
	if begins != 1 || ends != 1 || start >= end || !agentMarkerLine(text, start, agentBegin) || !agentMarkerLine(text, end, agentEnd) {
		return nil, fmt.Errorf("AGENTS.md has malformed or duplicate SeamGraph markers; keep exactly one begin/end pair on separate lines before setup")
	}
	if strings.Contains(text[start:end], "\r\n") {
		block = strings.ReplaceAll(block, "\n", "\r\n")
	}
	block = strings.TrimRight(block, "\r\n")
	return []byte(text[:start] + block + text[end+len(agentEnd):]), nil
}

func agentMarkerLine(text string, index int, marker string) bool {
	if index < 0 {
		return false
	}
	if index > 0 && text[index-1] != '\n' {
		return false
	}
	rest := text[index+len(marker):]
	return rest == "" || strings.HasPrefix(rest, "\n") || strings.HasPrefix(rest, "\r\n")
}

func writeAgentFile(scope *os.Root, file agentFile) error {
	if err := scope.MkdirAll(filepath.Dir(file.path), 0755); err != nil {
		return err
	}
	temporary := filepath.Join(filepath.Dir(file.path), ".seamgraph-"+rand.Text())
	defer func() { _ = scope.Remove(temporary) }()
	// #nosec G703 -- The random temporary filename is confined by os.Root to the selected project; exclusive creation avoids following a pre-existing symlink.
	f, err := scope.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, file.mode)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(file.after)
	if writeErr == nil {
		writeErr = f.Chmod(file.mode)
	}
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return scope.Rename(temporary, file.path)
}
