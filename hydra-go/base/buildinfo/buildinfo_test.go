package buildinfo

import "testing"

func TestString_DefaultsToDevWhenEmpty(t *testing.T) {
	old := Version
	Version = "   "
	t.Cleanup(func() {
		Version = old
	})

	if got := String(); got != "dev" {
		t.Fatalf("String() = %q, want %q", got, "dev")
	}
}

func TestCLIString_UsesNormalizedVersion(t *testing.T) {
	old := Version
	Version = " v1.2.3 "
	t.Cleanup(func() {
		Version = old
	})

	if got := CLIString(); got != "hydra v1.2.3" {
		t.Fatalf("CLIString() = %q, want %q", got, "hydra v1.2.3")
	}
}
