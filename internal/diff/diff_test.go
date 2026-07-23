package diff

import "testing"

func TestParseNumstat(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    Stats
	}{
		{
			name:   "single text file",
			output: "3\t1\tmain.go\n",
			want: Stats{
				FilesChanged: 1,
				Insertions:   3,
				Deletions:    1,
				ChangedFiles: []string{"main.go"},
			},
		},
		{
			name:   "multiple files",
			output: "10\t2\ta.go\n5\t0\tb.go\n",
			want: Stats{
				FilesChanged: 2,
				Insertions:   15,
				Deletions:    2,
				ChangedFiles: []string{"a.go", "b.go"},
			},
		},
		{
			name:   "binary file uses dash markers",
			output: "-\t-\tlogo.png\n",
			want: Stats{
				FilesChanged: 1,
				Insertions:   0,
				Deletions:    0,
				ChangedFiles: []string{"logo.png"},
			},
		},
		{
			name:   "empty output means no staged changes",
			output: "",
			want:   Stats{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseNumstat(tt.output)
			if err != nil {
				t.Fatalf("parseNumstat returned error: %v", err)
			}
			if got.FilesChanged != tt.want.FilesChanged ||
				got.Insertions != tt.want.Insertions ||
				got.Deletions != tt.want.Deletions ||
				len(got.ChangedFiles) != len(tt.want.ChangedFiles) {
				t.Errorf("parseNumstat(%q) = %+v, want %+v", tt.output, got, tt.want)
			}
		})
	}
}

func TestStats_FallbackMessage(t *testing.T) {
	s := Stats{FilesChanged: 3, Insertions: 12, Deletions: 4}
	got := s.FallbackMessage()
	want := "chore: update 3 file(s) (+12/-4)"
	if got != want {
		t.Errorf("FallbackMessage() = %q, want %q", got, want)
	}
}

func TestStats_TotalLines(t *testing.T) {
	s := Stats{Insertions: 7, Deletions: 3}
	if s.TotalLines() != 10 {
		t.Errorf("TotalLines() = %d, want 10", s.TotalLines())
	}
}
