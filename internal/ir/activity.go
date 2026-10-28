package ir

// Activity counts committed history separately from the final comparison trees.
type LineStats struct {
	Added   int  `json:"added"`
	Deleted int  `json:"deleted"`
	Binary  bool `json:"binary"`
}
type ReviewCommit struct {
	Date     string               `json:"date"`
	Parent   string               `json:"parent,omitempty"`
	Revision string               `json:"revision"`
	Message  string               `json:"message"`
	Type     string               `json:"type"`
	Scope    string               `json:"scope"`
	Breaking bool                 `json:"breaking"`
	Files    map[string]LineStats `json:"files"`
	Patch    string               `json:"patch,omitempty"`
}
type Activity struct {
	Base        string               `json:"base"`
	Head        string               `json:"head"`
	Policy      string               `json:"policy"`
	Unavailable string               `json:"unavailable,omitempty"`
	Commits     []ReviewCommit       `json:"commits"`
	Net         map[string]LineStats `json:"net"`
	Patch       string               `json:"patch,omitempty"`
}
