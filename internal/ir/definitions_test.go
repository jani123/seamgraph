package ir

import (
	"bytes"
	"testing"
)

func TestDefinitionIdentityValidationAndArchitectureEquivalence(t *testing.T) {
	original := fixture(t, "go")
	architecture, err := original.ArchitectureJSON()
	if err != nil {
		t.Fatal(err)
	}
	s, err := original.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	id := s.Symbols[0].ID
	location := s.Relations[0].Evidence[0]
	s.Symbols[0].Definition = &Definition{Name: "Read", Location: location, Documentation: "docs", Digest: "body"}
	got, err := s.ArchitectureJSON()
	if err != nil || !bytes.Equal(got, architecture) {
		t.Fatal("declarations changed architectural equivalence", err)
	}
	copy, err := s.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	copy.Symbols[0].Definition.Documentation = "changed"
	if s.Symbols[0].Definition.Documentation != "docs" {
		t.Fatal("canonical definition aliases input")
	}
	s.Symbols[0].Definition.ParentID = id
	if s.Validate() == nil {
		t.Fatal("accepted cyclic parent")
	}
	s.Symbols[0].Definition.ParentID = "symbol:missing:parent"
	if s.Validate() == nil {
		t.Fatal("accepted unknown parent")
	}
	s.Symbols[0].Definition.ParentID = ""
	s.Relations[0].Evidence[0].SourceID = "symbol:missing:source"
	if s.Validate() == nil {
		t.Fatal("accepted unknown source")
	}
	s.Relations[0].Evidence[0].SourceID = id
	s.Relations[0].Evidence[0].Kind = "runtime"
	if s.Validate() == nil {
		t.Fatal("accepted unsupported reference certainty")
	}
}
