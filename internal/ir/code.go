package ir

// CodeIndex combines declaration facts with optional, independently analyzed
// architecture context. Synthetic ownership is never an architecture suggestion.
type CodeIndex struct {
	Facts           Snapshot `json:"facts"`
	DeclaredModules []Module `json:"declaredModules"`
	DeclaredSeams   []Seam   `json:"declaredSeams"`
	Issues          []string `json:"issues"`
}
