package ir

import "fmt"

// Parent and source identities are validated after collecting every declaration.
func validateDefinitions(snapshot Snapshot, symbols map[string]Symbol) error {
	for _, symbol := range snapshot.Symbols {
		if err := validateParent(symbol, symbols); err != nil {
			return err
		}
	}
	for _, relation := range snapshot.Relations {
		for _, evidence := range relation.Evidence {
			if evidence.SourceID != "" {
				if _, ok := symbols[evidence.SourceID]; !ok {
					return fmt.Errorf("unknown reference source %q", evidence.SourceID)
				}
			}
		}
	}
	return nil
}
func validateParent(symbol Symbol, symbols map[string]Symbol) error {
	seen := map[string]bool{symbol.ID: true}
	for symbol.Definition != nil && symbol.Definition.ParentID != "" {
		id := symbol.Definition.ParentID
		parent, ok := symbols[id]
		if !ok || parent.ModuleID != symbol.ModuleID {
			return fmt.Errorf("invalid declaration parent %q", id)
		}
		if seen[id] {
			return fmt.Errorf("cyclic declaration parent %q", id)
		}
		seen[id] = true
		symbol = parent
	}
	return nil
}
