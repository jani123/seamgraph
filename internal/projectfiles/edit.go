package projectfiles

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"seamgraph/internal/config"
)

type Edit struct {
	Before []byte
	After  []byte
	Diff   string
}

func PrepareRule(root, effect, from, to string) (Edit, error) {
	var edit Edit
	if effect != "allow" && effect != "deny" {
		return edit, fmt.Errorf("rule effect must be allow or deny")
	}
	// #nosec G304 -- The configuration filename is constant and root is the explicitly selected project.
	b, err := os.ReadFile(filepath.Join(root, config.Filename))
	if err != nil {
		return edit, err
	}
	d, err := config.Parse(b)
	if err != nil {
		return edit, err
	}
	from = strings.TrimPrefix(from, "module:")
	to = strings.TrimPrefix(strings.TrimPrefix(to, "module:"), "seam:")
	r := config.Rule{From: from, To: to}
	if effect == "allow" {
		d.Config.Policies.Allow = append(d.Config.Policies.Allow, r)
	} else {
		d.Config.Policies.Deny = append(d.Config.Policies.Deny, r)
	}
	if err = d.Config.Validate(); err != nil {
		return edit, err
	}
	return prepareRuleText(b, d, effect, r)
}

// Commit checks the exact original bytes, preserves file permissions, saves a
// backup, and replaces atomically. Callers serialize edits within their process.
func (e Edit) Commit(root string) error {
	p := filepath.Join(root, config.Filename)
	info, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("configuration must be a regular file")
	}
	// #nosec G304 -- Commit rereads only the fixed configuration path and checks its expected contents.
	current, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if sha256.Sum256(current) != sha256.Sum256(e.Before) {
		return fmt.Errorf("configuration changed since preview; propose again")
	}
	if _, err = config.Parse(e.After); err != nil {
		return err
	}
	backup := p + ".bak"
	backupFile, err := os.CreateTemp(root, ".seamgraph-backup-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(backupFile.Name()) }()
	if err = backupFile.Chmod(info.Mode().Perm()); err != nil {
		_ = backupFile.Close()
		return err
	}
	if _, err = backupFile.Write(current); err != nil {
		_ = backupFile.Close()
		return err
	}
	if err = backupFile.Close(); err != nil {
		return err
	}
	if err = os.Rename(backupFile.Name(), backup); err != nil {
		return err
	}
	f, err := os.CreateTemp(root, ".seamgraph-config-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if err = f.Chmod(info.Mode().Perm()); err != nil {
		_ = f.Close()
		return err
	}
	if _, err = f.Write(e.After); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	// #nosec G304 -- Final concurrent-edit check rereads only the fixed configuration path.
	current, err = os.ReadFile(p)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, e.Before) {
		return fmt.Errorf("configuration changed during write; propose again")
	}
	return os.Rename(f.Name(), p)
}

func (e Edit) Preview() string { return e.Diff }

func prepareRuleText(b []byte, d *config.Document, effect string, r config.Rule) (Edit, error) {
	policies, policyEnd, err := policyMapping(b, d)
	if err != nil {
		return Edit{}, err
	}
	newline := "\n"
	if bytes.Contains(b, []byte("\r\n")) {
		newline = "\r\n"
	}
	lines := strings.SplitAfter(string(b), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if policyEnd > len(lines) {
		policyEnd = len(lines)
	}
	key, seq, start, end := ruleInsertion(policies, effect, lines, policyEnd)
	item := &yaml.Node{}
	if err = item.Encode(r); err != nil {
		return Edit{}, err
	}
	seq.Content = append(seq.Content, item)
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{key, seq}}
	var encoded bytes.Buffer
	encoder := yaml.NewEncoder(&encoded)
	encoder.SetIndent(2)
	if err = encoder.Encode(node); err != nil {
		return Edit{}, err
	}
	if err = encoder.Close(); err != nil {
		return Edit{}, err
	}
	insertion := ""
	for _, line := range strings.Split(strings.TrimSuffix(encoded.String(), "\n"), "\n") {
		insertion += "  " + line + newline
	}
	prefix := strings.Join(lines[:start], "")
	if prefix != "" && !strings.HasSuffix(prefix, "\n") {
		prefix += newline
	}
	after := []byte(prefix + insertion + strings.Join(lines[end:], ""))
	if _, err = config.Parse(after); err != nil {
		return Edit{}, fmt.Errorf("invalid proposed config: %w", err)
	}
	diff := "--- seamgraph.yaml\n+++ seamgraph.yaml\n"
	for _, line := range lines[start:end] {
		diff += "-" + strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r") + "\n"
	}
	for _, line := range strings.Split(strings.TrimSuffix(insertion, newline), newline) {
		diff += "+" + line + "\n"
	}
	return Edit{Before: b, After: after, Diff: diff}, nil
}

func policyMapping(b []byte, d *config.Document) (*yaml.Node, int, error) {
	mapping := d.Node.Content[0]
	var policies *yaml.Node
	policyEnd := len(strings.SplitAfter(string(b), "\n"))
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == "policies" {
			policies = mapping.Content[i+1]
			if i+2 < len(mapping.Content) {
				policyEnd = mapping.Content[i+2].Line - 1
			}
			break
		}
	}
	if policies == nil || policies.Style&yaml.FlowStyle != 0 {
		return nil, 0, fmt.Errorf("safe editing requires a block-style policies mapping")
	}
	return policies, policyEnd, nil
}

func ruleInsertion(policies *yaml.Node, effect string, lines []string, policyEnd int) (*yaml.Node, *yaml.Node, int, int) {
	start, end := policyEnd, policyEnd
	key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: effect}
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for i := 0; i < len(policies.Content); i += 2 {
		if policies.Content[i].Value == effect {
			start = policies.Content[i].Line - 1
			end = policyEnd
			if i+2 < len(policies.Content) {
				end = policies.Content[i+2].Line - 1
			}
			seq = policies.Content[i+1]
			copied := *policies.Content[i]
			copied.HeadComment = ""
			key = &copied
			break
		}
	}
	for end > start && (strings.TrimSpace(lines[end-1]) == "" || strings.HasPrefix(strings.TrimSpace(lines[end-1]), "#")) {
		end--
	}
	if start == policyEnd {
		start = end
	}
	return key, seq, start, end
}
