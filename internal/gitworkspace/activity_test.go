package gitworkspace

import "testing"

func TestNumstatKeepsBinaryAndUnusualPathsExplicit(t *testing.T) {
	stats, err := parseNumstat([]byte("2\t3\tstorefront/checkout.ts\x00-\t-\tlogo.bin\x001\t0\tname\twith\nnewline.txt\x00"))
	if err != nil || len(stats) != 3 || stats["storefront/checkout.ts"].Added != 2 || stats["storefront/checkout.ts"].Deleted != 3 || !stats["logo.bin"].Binary || stats["name\twith\nnewline.txt"].Added != 1 {
		t.Fatal(stats, err)
	}
	for _, input := range []string{"invalid\x00", "1\t2\t../outside\x00", "-1\t2\tfile\x00", "1\tbogus\tfile\x00", "-\t1\tfile\x00"} {
		if _, err := parseNumstat([]byte(input)); err == nil {
			t.Fatalf("accepted invalid stats %q", input)
		}
	}
}

func TestCommitClassificationDoesNotGuessUnmatchedSubjects(t *testing.T) {
	for _, test := range []struct {
		message, kind, scope string
		breaking             bool
	}{
		{"fix(checkout): clarify label\n", "fix", "checkout", false},
		{"feat(payment)!: widen amount", "feat", "payment", true},
		{"assets: add icon\n\nBREAKING CHANGE: binary marker", "assets", "", true},
		{"Change adapter\n\nBREAKING-CHANGE: new contract", "unmatched", "", true},
		{" fix: leading whitespace is not a conventional subject", "unmatched", "", false},
		{"fix:missing subject separator", "unmatched", "", false},
		{"Merge diverging target", "unmatched", "", false},
	} {
		kind, scope, breaking := classifyCommit(test.message)
		if kind != test.kind || scope != test.scope || breaking != test.breaking {
			t.Fatalf("%q: got %s/%s/%v", test.message, kind, scope, breaking)
		}
	}
}
