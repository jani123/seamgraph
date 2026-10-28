package ir

// ReviewRange records both the history target and the two compared trees.
type ReviewRange struct {
	Mode           string `json:"mode"`
	Target         string `json:"target"`
	Review         string `json:"review"`
	TargetRevision string `json:"targetRevision"`
	Base           string `json:"base"`
	Head           string `json:"head"`
}

// BranchChoices contains locally available refs; listing never fetches a remote.
type BranchChoice struct {
	Ref     string `json:"ref"`
	Name    string `json:"name"`
	Remote  bool   `json:"remote"`
	Current bool   `json:"current"`
}
type BranchChoices struct {
	Items       []BranchChoice `json:"items"`
	Current     string         `json:"current"`
	Default     string         `json:"default"`
	Unavailable string         `json:"unavailable,omitempty"`
}
