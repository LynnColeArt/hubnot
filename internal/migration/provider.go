package migration

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// ProviderConfig contains explicit connection inputs. Token is ephemeral: never
// serialize it into snapshots, plans or progress. Constructors must be inert.
// Attachments are explicit selections, not automatically followed body links.
type ProviderConfig struct {
	Locator           Locator
	Token             string `json:"-"`
	APIVersion        string
	CredentialMode    string
	WorkflowRef       string
	PlanningSelectors []string
	Attachments       []AttachmentSelection
}
type AttachmentSelection struct {
	URL, Name string
	Parent    Key
}
type ProviderRegistration struct {
	Kind             string
	TokenEnv         string
	NewSource        func(ProviderConfig) (Source, error)
	NewDestination   func(ProviderConfig) (Destination, error)
	RequiredCoverage []CoverageRequirement
}

// Registry supports disjoint optional provider files without native-engine
// conditionals. Registration and lookup never contact the provider.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]ProviderRegistration
}

func (r *Registry) Register(p ProviderRegistration) error {
	if !text(p.Kind, 128, true) || p.NewSource == nil || p.NewDestination == nil || len(p.RequiredCoverage) == 0 {
		return fmt.Errorf("provider needs a kind and source/destination constructors")
	}
	for _, c := range p.Kind {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return fmt.Errorf("invalid provider kind")
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = map[string]ProviderRegistration{}
	}
	if _, ok := r.entries[p.Kind]; ok {
		return fmt.Errorf("provider already registered: %s", p.Kind)
	}
	p.RequiredCoverage = append([]CoverageRequirement{}, p.RequiredCoverage...)
	r.entries[p.Kind] = p
	return nil
}
func (r *Registry) Lookup(kind string) (ProviderRegistration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.entries[kind]
	p.RequiredCoverage = append([]CoverageRequirement{}, p.RequiredCoverage...)
	return p, ok
}
func (r *Registry) Kinds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []string{}
	for k := range r.entries {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ValidateProviderCoverage makes the named provider baseline nonvacuous. It is
// shared by registration consumers and fixture conformance tests. Extra kinds
// must declare either actual reads/preservation or a reason for unavailability.
// CoverageRequirement belongs to an adapter profile, not the engine. Providers
// with different API models declare their actual baseline in their own files.
type CoverageRequirement struct {
	Kind                           string
	Read, Preserve, Create, Update bool
}

func ValidateProviderCoverage(required []CoverageRequirement, source, destination []Capability) error {
	index := func(cs []Capability) (map[string]Capability, error) {
		m := map[string]Capability{}
		for _, c := range cs {
			if c.Kind == "" {
				return nil, fmt.Errorf("empty capability")
			}
			if _, ok := m[c.Kind]; ok {
				return nil, fmt.Errorf("duplicate capability")
			}
			if !c.Read && !c.Create && !c.Update && !c.Preserve && c.Reason == "" {
				return nil, fmt.Errorf("unavailable capability needs reason")
			}
			if c.Preserve && !c.Read {
				return nil, fmt.Errorf("preservation needs retrieved content")
			}
			m[c.Kind] = c
		}
		return m, nil
	}
	s, e := index(source)
	if e != nil {
		return e
	}
	d, e := index(destination)
	if e != nil {
		return e
	}
	if len(required) == 0 {
		return fmt.Errorf("provider needs nonempty coverage baseline")
	}
	useful := false
	for _, r := range required {
		if r.Kind == "" {
			return fmt.Errorf("empty coverage kind")
		}
		useful = useful || r.Read || r.Preserve || r.Create || r.Update
		if r.Read && !s[r.Kind].Read || r.Preserve && !s[r.Kind].Preserve || r.Create && !d[r.Kind].Create || r.Update && !d[r.Kind].Update {
			return fmt.Errorf("missing required %s coverage", r.Kind)
		}
	}
	if !useful {
		return fmt.Errorf("all-unavailable coverage baseline")
	}
	return nil
}

// Open checks actual declared capabilities against this adapter's baseline.
// Both constructors and Capabilities are offline metadata operations; no request
// or credential discovery is permitted here. Snapshot/Plan are explicit reads.
func (p ProviderRegistration) Open(ctx context.Context, c ProviderConfig) (Source, Destination, error) {
	if c.Locator.Kind != p.Kind {
		return nil, nil, fmt.Errorf("provider kind differs from configured locator")
	}
	s, e := p.NewSource(c)
	if e != nil {
		return nil, nil, e
	}
	d, e := p.NewDestination(c)
	if e != nil {
		return nil, nil, e
	}
	if s == nil || d == nil {
		return nil, nil, fmt.Errorf("provider returned nil source or destination")
	}
	sc, e := s.Capabilities(ctx)
	if e != nil {
		return nil, nil, e
	}
	dc, e := d.Capabilities(ctx)
	if e != nil {
		return nil, nil, e
	}
	if e = ValidateProviderCoverage(p.RequiredCoverage, sc, dc); e != nil {
		return nil, nil, e
	}
	return s, d, nil
}
