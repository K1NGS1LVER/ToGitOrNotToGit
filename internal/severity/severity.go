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

func (t Tier) Persona() string {
	switch t {
	case Minor:
		return "victorian-gothic"
	case Catastrophic:
		return "shakespearean-tragedy"
	default:
		return "soap-opera"
	}
}

var overrideBasenames = []string{"go.mod", "go.sum", "Dockerfile", "Makefile"}

func isOverrideFile(path string) bool {
	base := filepath.Base(path)
	for _, name := range overrideBasenames {
		if base == name {
			return true
		}
	}
	return strings.HasPrefix(path, ".github/workflows/")
}

func Score(stats diff.Stats) Tier {
	for _, f := range stats.ChangedFiles {
		if isOverrideFile(f) {
			return Catastrophic
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
