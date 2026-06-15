package contract

import (
	"strings"
	"testing"
)

func TestContractRejectsBannedAliases(t *testing.T) {
	reg := NewRegistry()
	reg.Add(Command{Name: "media info", Canonical: false})
	err := reg.Validate()
	if err == nil || !strings.Contains(err.Error(), "use get or probe, not info") {
		t.Fatalf("expected banned alias error, got %v", err)
	}
}

func TestDefaultRegistryIncludesCoreCommands(t *testing.T) {
	reg := DefaultRegistry()
	for _, name := range []string{"project init", "media probe", "timeline compile", "render transcript-cut", "nle export", "qa analyze"} {
		if _, ok := reg.Get(name); !ok {
			t.Fatalf("missing command %q", name)
		}
	}
	if err := reg.Validate(); err != nil {
		t.Fatalf("default registry should validate: %v", err)
	}
}
