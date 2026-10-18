package severity

import (
	"testing"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/diff"
)

func TestScore_LineCountTiers(t *testing.T) {
	tests := []struct {
		name       string
		insertions int
		deletions  int
		want       Tier
	}{
		{"nine lines is minor", 5, 4, Minor},
		{"ten lines is medium", 6, 4, Medium},
		{"hundred lines is medium", 60, 40, Medium},
		{"hundred one lines is catastrophic", 60, 41, Catastrophic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := diff.Stats{Insertions: tt.insertions, Deletions: tt.deletions}
			if got := Score(stats, nil); got != tt.want {
				t.Errorf("Score(%+v) = %v, want %v", stats, got, tt.want)
			}
		})
	}
}

func TestScore_CoreFileOverride(t *testing.T) {
	tests := []string{"go.mod", "go.sum", "Dockerfile", "Makefile", ".github/workflows/ci.yml"}
	for _, f := range tests {
		t.Run(f, func(t *testing.T) {
			stats := diff.Stats{Insertions: 1, Deletions: 0, ChangedFiles: []string{f}}
			if got := Score(stats, nil); got != Catastrophic {
				t.Errorf("Score with changed file %q = %v, want Catastrophic", f, got)
			}
		})
	}
}

func TestScore_CustomOverrides(t *testing.T) {
	stats := diff.Stats{Insertions: 1, ChangedFiles: []string{"scripts/release.sh"}}
	if got := Score(stats, []string{"scripts/*"}); got != Catastrophic {
		t.Errorf("Score with glob override = %v, want Catastrophic", got)
	}

	// A non-override file with typical size stays Minor.
	small := diff.Stats{Insertions: 1, ChangedFiles: []string{"main.go"}}
	if got := Score(small, []string{"scripts/*"}); got != Minor {
		t.Errorf("Score(small) = %v, want Minor", got)
	}
}

func TestScore_OverridesReplaceDefaults(t *testing.T) {
	// go.mod is catastrophic by default...
	stats := diff.Stats{Insertions: 1, ChangedFiles: []string{"go.mod"}}
	if got := Score(stats, nil); got != Catastrophic {
		t.Fatalf("default go.mod = %v, want Catastrophic", got)
	}
	// ...but an explicit override list that omits it drops go.mod back to line-based scoring.
	if got := Score(stats, []string{"Dockerfile"}); got != Minor {
		t.Errorf("go.mod with custom overrides = %v, want Minor", got)
	}
}

func TestIsValidPersona(t *testing.T) {
	for _, p := range ValidPersonas() {
		if !IsValidPersona(p) {
			t.Errorf("IsValidPersona(%q) = false, want true", p)
		}
	}
	for _, bad := range []string{"", "victorian", "Soap Opera", "unknown"} {
		if IsValidPersona(bad) {
			t.Errorf("IsValidPersona(%q) = true, want false", bad)
		}
	}
}

func TestTier_Persona(t *testing.T) {
	tests := []struct {
		tier Tier
		want string
	}{
		{Minor, "victorian-gothic"},
		{Medium, "soap-opera"},
		{Catastrophic, "shakespearean-tragedy"},
	}
	for _, tt := range tests {
		if got := tt.tier.Persona(); got != tt.want {
			t.Errorf("Tier(%d).Persona() = %q, want %q", tt.tier, got, tt.want)
		}
	}
}
