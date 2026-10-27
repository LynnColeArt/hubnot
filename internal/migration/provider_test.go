package migration

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestRegistryDisjointRegistrationIsInert(t *testing.T) {
	var r Registry
	calls := 0
	p := ProviderRegistration{Kind: "fixture", RequiredCoverage: []CoverageRequirement{{Kind: "issue", Read: true}}, NewSource: func(ProviderConfig) (Source, error) { calls++; return nil, nil }, NewDestination: func(ProviderConfig) (Destination, error) { calls++; return nil, nil }}
	if e := r.Register(p); e != nil {
		t.Fatal(e)
	}
	if e := r.Register(p); e == nil {
		t.Fatal("duplicate registration accepted")
	}
	if len(r.Kinds()) != 1 || calls != 0 {
		t.Fatal("registration performed work")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if _, ok := r.Lookup("fixture"); !ok {
					t.Error("missing registered provider")
				}
			}
		}()
	}
	wg.Wait()
	p, _ = r.Lookup("fixture")
	if _, e := p.NewSource(ProviderConfig{}); e != nil {
		t.Fatal(e)
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}

func TestProviderConfigNeverSerializesCredential(t *testing.T) {
	raw, e := json.Marshal(ProviderConfig{Token: "private-provider-token", CredentialMode: "bearer", APIVersion: "2022-11-28"})
	if e != nil || strings.Contains(string(raw), "private-provider-token") {
		t.Fatal("ephemeral credential serialized", string(raw), e)
	}
}
