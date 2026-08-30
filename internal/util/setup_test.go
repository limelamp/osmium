package util

import "testing"

// The engine's name reaches this package from a wizard, a manifest, a modpack's
// index and a command-line flag, and each of them capitalises it differently.
// Matching only one spelling meant "osmium install" on a modpack downloaded
// nothing and reported an empty URL.
func TestAnEngineIsRecognisedWhateverItsCase(t *testing.T) {
	for _, given := range []string{"fabric", "Fabric", "FABRIC", " fabric "} {
		if got := CanonicalLoader(given); got != "Fabric" {
			t.Errorf("%q is read as %q, want Fabric", given, got)
		}
	}

	for _, given := range []string{"neoforge", "NeoForge", "NEOFORGE"} {
		if got := CanonicalLoader(given); got != "NeoForge" {
			t.Errorf("%q is read as %q, want NeoForge", given, got)
		}
	}

	// Something nobody supports comes back as it was, for the error to name.
	if got := CanonicalLoader("sponge"); got != "sponge" {
		t.Errorf("an unknown engine came back as %q", got)
	}
}
