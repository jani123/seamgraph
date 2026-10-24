package ui

import (
	"net/url"
	"testing"

	"seamgraph/internal/ir"
)

func TestHistoryFileLinksUseExactCommitComparison(t *testing.T) {
	for _, parent := range []string{"parent-sha", ""} {
		address, err := url.Parse(historySourceLink(ir.ReviewCommit{Revision: "commit-sha", Parent: parent}, "file with & spaces.txt"))
		if err != nil {
			t.Fatal(err)
		}
		query := address.Query()
		if address.Path != "/explore" || query.Get("revision") != "commit-sha" || query.Get("base") != parent || query.Get("file") != "file with & spaces.txt" || query.Get("changes") != "1" {
			t.Fatal(address)
		}
		if query.Get("tools") != "" {
			t.Fatal("history file link must open Source")
		}
	}
}
