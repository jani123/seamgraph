// Package application owns project use cases. Infrastructure is supplied through
// consumer-owned ports; no workflow constructs a parser, process or file store.
package application

import (
	"context"
	"path/filepath"
	"seamgraph/internal/config"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
	"seamgraph/internal/verification"
	"slices"
	"sort"
)

type Model struct {
	Snapshot  ir.Snapshot         `json:"snapshot"`
	Check     policy.Result       `json:"check"`
	Contracts verification.Report `json:"contracts"`
	Policy    policy.Policy       `json:"policy"`
}
type Workspace struct {
	Root   string `json:"root"`
	URL    string `json:"url"`
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
}
type EvidenceRun struct{ Runner, Revision, Fingerprint, RunID, ObservedAt string }

// RuleEdit keeps prepared bytes private to the file implementation.
type RuleEdit interface {
	Preview() string
	Commit(string) error
}

// Ports are capabilities supplied once at construction. Queries never call the
// mutation ports. Extract must return cleanup even when materialization fails.
type Ports struct {
	Branches        func(context.Context, string) (ir.BranchChoices, error)
	NetStats        func(context.Context, string, string, string) (map[string]ir.LineStats, error)
	BranchRange     func(context.Context, string, string, string, string) (ir.ReviewRange, error)
	Index           func(context.Context, string, map[string]string) (ir.CodeIndex, error)
	BrowseInventory func(string) (map[string]string, error)
	Activity        func(context.Context, string, string, string, string) (ir.Activity, error)
	ReadReview      func(string) ([]ReviewEntry, error)
	AppendReview    func(string, ReviewEntry) error
	ConfigKey       func(string) (string, error)
	Inventory       func(string) (map[string]string, error)
	Scan            func(context.Context, string) (ir.Snapshot, config.Config, error)
	LoadConfig      func(string) (*config.Document, error)
	LoadContracts   func(string) (verification.Manifest, error)
	ReadStore       func(string) (verification.Store, error)
	AppendEvidence  func(string, []verification.Evidence) (verification.Store, error)
	EvidenceDir     func(string) (string, error)
	ReadFile        func(string) ([]byte, error)
	Execute         func(context.Context, string, string, string) ([]byte, string, error, error)
	Import          func(string, []byte, EvidenceRun) ([]verification.Evidence, error)
	Identity        func(context.Context, string) (string, string, error)
	Extract         Extract
	Open            func(context.Context, string, string, string) (Workspace, error)
	PrepareProject  func(string, []byte) (bool, error)
	Initialize      func(string) error
	SetupAgent      func(string) ([]string, error)
	PrepareRule     func(string, string, string, string) (RuleEdit, error)
	Source          func(string, string, []ir.Evidence) (string, error)
}
type Service struct {
	code  codeCache
	root  string
	ports Ports
	cache Cache
	rules proposals
}

func New(root string, ports Ports, cacheContext string) (*Service, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Service{root: root, ports: ports, cache: Cache{Context: cacheContext, ConfigKey: ports.ConfigKey, Inventory: ports.Inventory}}, nil
}
func (s *Service) At(root string) (*Service, error) { return New(root, s.ports, s.cache.Context) }
func (s *Service) Root() string                     { return s.root }

// ConfigFilename is the application-facing project document name.
const ConfigFilename = config.Filename

func ValidateConfiguration(data []byte) error { _, err := config.Parse(data); return err }
func (s *Service) Validate(root string) (int, error) {
	doc, err := s.ports.LoadConfig(root)
	if err != nil {
		return 0, err
	}
	return len(doc.Config.Modules), nil
}
func (s *Service) ConfiguredLanguages(root string) ([]string, error) {
	doc, err := s.ports.LoadConfig(root)
	if err != nil {
		return nil, err
	}
	languages := make([]string, 0, len(doc.Config.Modules))
	for _, module := range doc.Config.Modules {
		languages = append(languages, module.Language)
	}
	sort.Strings(languages)
	return slices.Compact(languages), nil
}
func (s *Service) Initialize(root string) error             { return s.ports.Initialize(root) }
func (s *Service) SetupAgent(root string) ([]string, error) { return s.ports.SetupAgent(root) }
func (s *Service) Identity(ctx context.Context, root string) (string, string, error) {
	return s.ports.Identity(ctx, root)
}
func (s *Service) Open(ctx context.Context, url, ref, cache string, supplied []byte) (Workspace, bool, error) {
	w, err := s.ports.Open(ctx, url, ref, cache)
	if err != nil {
		return w, false, err
	}
	created, err := s.ports.PrepareProject(w.Root, supplied)
	return w, created, err
}
func (s *Service) analyze(ctx context.Context, root string) (ir.Snapshot, policy.Policy, error) {
	graph, c, err := s.ports.Scan(ctx, root)
	if err != nil {
		return graph, policy.Policy{}, err
	}
	p, err := NormalizePolicy(c)
	return graph, p, err
}
func (s *Service) check(graph ir.Snapshot, p policy.Policy) (policy.Result, error) {
	return policy.Evaluate(graph, p)
}
func (s *Service) Load(ctx context.Context) (Model, error) {
	graph, p, err := s.analyze(ctx, s.root)
	if err != nil {
		return Model{}, err
	}
	check, err := s.check(graph, p)
	if err != nil {
		return Model{}, err
	}
	contracts, err := s.contractReport(ctx, s.root, graph)
	return Model{graph, check, contracts, p}, err
}
func (s *Service) Compare(ctx context.Context, base, head string) (Result, error) {
	return s.cache.Compare(ctx, s.root, base, head, s.ports.Extract, s.analyze, s.revisionContracts(s.root))
}
