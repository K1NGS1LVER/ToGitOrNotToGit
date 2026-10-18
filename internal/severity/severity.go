package severity

import (
	"path/filepath"
	"strings"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/diff"
)

type Tier int

const (
	Minor Tier = iota
	Medium
	Catastrophic
)

// Persona names accepted by the CLI flag and the config file. Kept here (not in
// cmd) so both cmd and config can validate against the same source of truth.
const (
	PersonaVictorianGothic = "victorian-gothic"
	PersonaSoapOpera       = "soap-opera"
	PersonaShakespearean   = "shakespearean-tragedy"
)

func (t Tier) Persona() string {
	switch t {
	case Minor:
		return PersonaVictorianGothic
	case Catastrophic:
		return PersonaShakespearean
	default:
		return PersonaSoapOpera
	}
}

// ValidPersonas returns every persona name tocommit accepts.
func ValidPersonas() []string {
	return []string{PersonaVictorianGothic, PersonaSoapOpera, PersonaShakespearean}
}

// IsValidPersona reports whether name is a known persona. The empty string is
// not a persona; callers that treat "" as "auto-detect" must handle it first.
func IsValidPersona(name string) bool {
	for _, p := range ValidPersonas() {
		if p == name {
			return true
		}
	}
	return false
}

// DefaultOverridePatterns forces the Catastrophic tier for changes that touch
// dependency manifests, container/build files, or CI workflows. Used when the
// config does not supply its own SeverityOverrides.
func DefaultOverridePatterns() []string {
	return []string{"go.mod", "go.sum", "Dockerfile", "Makefile", ".github/workflows/*"}
}

// matchesOverride reports whether path matches any pattern. A pattern containing
// glob metacharacters is matched (with filepath.Match) against both the full path
// and its basename; a pattern ending in "/" is a directory prefix; anything else
// matches on basename equality.
func matchesOverride(path, pattern string) bool {
	if pattern == "" {
		return false
	}
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path, pattern)
	}
	if strings.ContainsAny(pattern, "*?[") {
		if ok, err := filepath.Match(pattern, path); err == nil && ok {
			return true
		}
		ok, err := filepath.Match(pattern, filepath.Base(path))
		return err == nil && ok
	}
	return filepath.Base(path) == pattern
}

// Score classifies a diff into a tier. overrides is a list of path patterns that
// force Catastrophic; an empty list falls back to DefaultOverridePatterns.
func Score(stats diff.Stats, overrides []string) Tier {
	patterns := overrides
	if len(patterns) == 0 {
		patterns = DefaultOverridePatterns()
	}
	for _, f := range stats.ChangedFiles {
		for _, p := range patterns {
			if matchesOverride(f, p) {
				return Catastrophic
			}
		}
	}

	switch total := stats.TotalLines(); {
	case total < 10:
		return Minor
	case total <= 100:
		return Medium
	default:
		return Catastrophic
	}
}
