package projectfiles

import (
	"bytes"
	"os"
	"path/filepath"
	"seamgraph/internal/config"
	"strings"
	"testing"
)

func TestRuleEditPreservesUnrelatedText(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		t.Run(strings.ReplaceAll(newline, "\n", "LF"), func(t *testing.T) {
			root := t.TempDir()
			data := strings.ReplaceAll(valid, "\n", newline)
			if err := os.WriteFile(filepath.Join(root, config.Filename), []byte(data), 0640); err != nil {
				t.Fatal(err)
			}
			edit, err := PrepareRule(root, "allow", "module:ui", "seam:world.reader")
			if err != nil {
				t.Fatal(err)
			}
			beforePolicies := strings.Split(data, "policies:")[0]
			if !bytes.HasPrefix(edit.After, []byte(beforePolicies)) {
				t.Fatal("unrelated modules reformatted")
			}
			current, _ := os.ReadFile(filepath.Join(root, config.Filename))
			if string(current) != data {
				t.Fatal("proposal wrote configuration")
			}
			if err = edit.Commit(root); err != nil {
				t.Fatal(err)
			}
			backup, _ := os.ReadFile(filepath.Join(root, config.Filename+".bak"))
			if string(backup) != data {
				t.Fatal("bad backup")
			}
			doc, err := Load(root)
			if err != nil || len(doc.Config.Policies.Allow) != 3 {
				t.Fatal(doc, err)
			}
			info, _ := os.Stat(filepath.Join(root, config.Filename))
			if info.Mode().Perm() != 0640 {
				t.Fatal("permissions changed")
			}
		})
	}
}
func TestNewRuleListAndConcurrentChange(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, config.Filename), []byte(valid), 0644); err != nil {
		t.Fatal(err)
	}
	edit, err := PrepareRule(root, "deny", "module:ui", "module:world")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = config.Parse(edit.After); err != nil {
		t.Fatal(err)
	}
	changed := []byte(valid + "# concurrent edit\n")
	if err = os.WriteFile(filepath.Join(root, config.Filename), changed, 0644); err != nil {
		t.Fatal(err)
	}
	if err = edit.Commit(root); err == nil {
		t.Fatal("stale edit committed")
	}
	after, _ := os.ReadFile(filepath.Join(root, config.Filename))
	if !bytes.Equal(after, changed) {
		t.Fatal("concurrent change lost")
	}
	for _, effect := range []string{"execute", "allow"} {
		to := "module:missing"
		if _, err = PrepareRule(root, effect, "module:ui", to); err == nil {
			t.Fatal("invalid proposal accepted")
		}
	}
}
