package diff

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const maxDiffBytes = 8000

type Stats struct {
	FilesChanged int
	Insertions   int
	Deletions    int
	ChangedFiles []string
}

func (s Stats) TotalLines() int {
	return s.Insertions + s.Deletions
}

func (s Stats) FallbackMessage() string {
	return fmt.Sprintf("chore: update %d file(s) (+%d/-%d)", s.FilesChanged, s.Insertions, s.Deletions)
}

// Collect runs git against the staged changes in the current working directory.
func Collect() (Stats, string, error) {
	numstatOut, err := runGit("diff", "--cached", "--numstat")
	if err != nil {
		return Stats{}, "", fmt.Errorf("running git diff --numstat: %w", err)
	}

	stats, err := parseNumstat(numstatOut)
	if err != nil {
		return Stats{}, "", err
	}

	rawDiff, err := runGit("diff", "--cached")
	if err != nil {
		return Stats{}, "", fmt.Errorf("running git diff: %w", err)
	}
	if len(rawDiff) > maxDiffBytes {
		rawDiff = rawDiff[:maxDiffBytes] + "\n... (truncated)"
	}

	return stats, rawDiff, nil
}

func parseNumstat(output string) (Stats, error) {
	var s Stats

	trimmed := strings.TrimRight(output, "\n")
	if trimmed == "" {
		return s, nil
	}

	for _, line := range strings.Split(trimmed, "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			return Stats{}, fmt.Errorf("unexpected numstat line: %q", line)
		}

		s.FilesChanged++
		s.ChangedFiles = append(s.ChangedFiles, fields[2])

		if fields[0] != "-" {
			n, err := strconv.Atoi(fields[0])
			if err != nil {
				return Stats{}, fmt.Errorf("parsing insertions in %q: %w", line, err)
			}
			s.Insertions += n
		}

		if fields[1] != "-" {
			n, err := strconv.Atoi(fields[1])
			if err != nil {
				return Stats{}, fmt.Errorf("parsing deletions in %q: %w", line, err)
			}
			s.Deletions += n
		}
	}

	return s, nil
}

func runGit(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
