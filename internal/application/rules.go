package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"seamgraph/internal/ir"
	"sync"
)

type Proposal struct {
	ID   string
	Diff string
}
type proposals struct {
	mu    sync.Mutex
	edits map[string]RuleEdit
}

// Propose previews without writing. Pending proposals are bounded per session.
func (s *Service) Propose(effect, from, to string) (Proposal, error) {
	s.rules.mu.Lock()
	defer s.rules.mu.Unlock()
	edit, err := s.ports.PrepareRule(s.root, effect, from, to)
	if err != nil {
		return Proposal{}, err
	}
	key := make([]byte, 24)
	if _, err = rand.Read(key); err != nil {
		return Proposal{}, err
	}
	if s.rules.edits == nil || len(s.rules.edits) >= 100 {
		s.rules.edits = map[string]RuleEdit{}
	}
	id := hex.EncodeToString(key)
	s.rules.edits[id] = edit
	return Proposal{id, edit.Preview()}, nil
}
func (s *Service) Cancel(id string) {
	s.rules.mu.Lock()
	defer s.rules.mu.Unlock()
	delete(s.rules.edits, id)
}

// Confirm consumes the proposal even on conflict; retries require a new preview.
func (s *Service) Confirm(id string) error {
	s.rules.mu.Lock()
	defer s.rules.mu.Unlock()
	edit, ok := s.rules.edits[id]
	delete(s.rules.edits, id)
	if !ok {
		return ErrUnknownProposal
	}
	return edit.Commit(s.root)
}
func (s *Service) Source(ctx context.Context, file string) (string, error) {
	model, err := s.Load(ctx)
	if err != nil {
		return "", err
	}
	locations := []ir.Evidence{}
	for _, relation := range model.Snapshot.Relations {
		locations = append(locations, relation.Evidence...)
	}
	for _, diagnostic := range model.Snapshot.Diagnostics {
		locations = append(locations, diagnostic.Evidence...)
	}
	for _, evidence := range model.Contracts.Evidence {
		if evidence.SourceLocation != nil {
			locations = append(locations, *evidence.SourceLocation)
		}
	}
	return s.ports.Source(s.root, file, locations)
}

var ErrUnknownProposal = errors.New("unknown/expired proposal")

type SourceError struct{ Kind, Message string }

func (e *SourceError) Error() string { return e.Message }
