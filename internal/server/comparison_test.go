package server

import (
	"net/http/httptest"
	"testing"
)

func TestComparisonScopes(t *testing.T) {
	for _, tc := range []struct{ query, head, base string }{
		{"scope=working&base=ignored&head=ignored", "WORKTREE", "HEAD"},
		{"scope=revisions&base=main&head=feature", "feature", "main"},
		{"base=main&head=feature", "feature", "main"},
		{"base=main&revision=feature", "feature", "main"},
		{"revision=WORKTREE", "", ""},
		{"base=HEAD&revision=WORKTREE", "WORKTREE", "HEAD"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			head, base := comparisonRefs(httptest.NewRequest("GET", "/?"+tc.query, nil))
			if head != tc.head || base != tc.base {
				t.Fatalf("got %q → %q, want %q → %q", base, head, tc.base, tc.head)
			}
		})
	}
}
