package application

import (
	"slices"
	"strings"

	"seamgraph/internal/config"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
)

// Policy normalizes YAML intent into canonical identities, dropping all language
// declarations before the rules engine sees it. Evaluate validates against IR.
func NormalizePolicy(c config.Config) (policy.Policy, error) {
	p := policy.Policy{Default: c.Policies.Default, Unresolved: c.Policies.Unresolved, Rules: []policy.Rule{}}
	if err := c.Validate(); err != nil {
		return p, err
	}
	for _, group := range []struct {
		effect string
		rules  []config.Rule
	}{{"allow", c.Policies.Allow}, {"deny", c.Policies.Deny}} {
		for _, r := range group.rules {
			target := strings.Split(r.To, ".")
			to := ir.ModuleID(target[0])
			if len(target) == 2 {
				to = ir.SeamID(target[0], target[1])
			}
			p.Rules = append(p.Rules, policy.Rule{FromID: ir.ModuleID(r.From), ToID: to, Effect: group.effect})
		}
	}
	slices.SortFunc(p.Rules, func(a, b policy.Rule) int { return strings.Compare(a.ID(), b.ID()) })
	return p, nil
}
