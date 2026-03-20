package workflow

import (
	"sort"
	"strings"
	"testing"
)

// levelIDs extracts sorted step IDs from each DAGLevel for deterministic comparison.
func levelIDs(levels []DAGLevel) [][]string {
	result := make([][]string, len(levels))
	for i, l := range levels {
		ids := make([]string, len(l.Steps))
		for j, s := range l.Steps {
			ids[j] = s.ID
		}
		sort.Strings(ids)
		result[i] = ids
	}
	return result
}

func equalLevels(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

func formatLevels(levels [][]string) string {
	parts := make([]string, len(levels))
	for i, l := range levels {
		parts[i] = "[" + strings.Join(l, ",") + "]"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func TestBuildDAG(t *testing.T) {
	tests := []struct {
		name       string
		steps      []Step
		wantLevels [][]string // sorted IDs per level
		wantErr    bool
		errContain string
	}{
		{
			name:       "empty steps",
			steps:      nil,
			wantLevels: nil,
		},
		{
			name:       "single step no deps",
			steps:      []Step{{ID: "A"}},
			wantLevels: [][]string{{"A"}},
		},
		{
			name: "linear chain A→B→C",
			steps: []Step{
				{ID: "A"},
				{ID: "B", DependsOn: []string{"A"}},
				{ID: "C", DependsOn: []string{"B"}},
			},
			wantLevels: [][]string{{"A"}, {"B"}, {"C"}},
		},
		{
			name: "fan-out A→{B,C,D}",
			steps: []Step{
				{ID: "A"},
				{ID: "B", DependsOn: []string{"A"}},
				{ID: "C", DependsOn: []string{"A"}},
				{ID: "D", DependsOn: []string{"A"}},
			},
			wantLevels: [][]string{{"A"}, {"B", "C", "D"}},
		},
		{
			name: "fan-in {B,C,D}→E",
			steps: []Step{
				{ID: "B"},
				{ID: "C"},
				{ID: "D"},
				{ID: "E", DependsOn: []string{"B", "C", "D"}},
			},
			wantLevels: [][]string{{"B", "C", "D"}, {"E"}},
		},
		{
			name: "diamond A→{B,C}→D",
			steps: []Step{
				{ID: "A"},
				{ID: "B", DependsOn: []string{"A"}},
				{ID: "C", DependsOn: []string{"A"}},
				{ID: "D", DependsOn: []string{"B", "C"}},
			},
			wantLevels: [][]string{{"A"}, {"B", "C"}, {"D"}},
		},
		{
			name: "complex DAG A→{B,C}, B→D, C→D, D→E",
			steps: []Step{
				{ID: "A"},
				{ID: "B", DependsOn: []string{"A"}},
				{ID: "C", DependsOn: []string{"A"}},
				{ID: "D", DependsOn: []string{"B", "C"}},
				{ID: "E", DependsOn: []string{"D"}},
			},
			wantLevels: [][]string{{"A"}, {"B", "C"}, {"D"}, {"E"}},
		},
		{
			name: "multiple roots {A,B}→C",
			steps: []Step{
				{ID: "A"},
				{ID: "B"},
				{ID: "C", DependsOn: []string{"A", "B"}},
			},
			wantLevels: [][]string{{"A", "B"}, {"C"}},
		},
		{
			name: "cycle A↔B",
			steps: []Step{
				{ID: "A", DependsOn: []string{"B"}},
				{ID: "B", DependsOn: []string{"A"}},
			},
			wantErr:    true,
			errContain: "cycle detected",
		},
		{
			name: "self-loop A→A",
			steps: []Step{
				{ID: "A", DependsOn: []string{"A"}},
			},
			wantErr:    true,
			errContain: "cycle detected",
		},
		{
			name: "3-node cycle A→B→C→A",
			steps: []Step{
				{ID: "A", DependsOn: []string{"C"}},
				{ID: "B", DependsOn: []string{"A"}},
				{ID: "C", DependsOn: []string{"B"}},
			},
			wantErr:    true,
			errContain: "cycle detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			levels, err := BuildDAG(tt.steps)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContain)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := levelIDs(levels)
			if !equalLevels(got, tt.wantLevels) {
				t.Errorf("levels mismatch\ngot:  %s\nwant: %s", formatLevels(got), formatLevels(tt.wantLevels))
			}

			// Verify Depth fields are correct.
			for i, l := range levels {
				if l.Depth != i {
					t.Errorf("level %d has Depth=%d, want %d", i, l.Depth, i)
				}
			}
		})
	}
}

func TestValidateDAG(t *testing.T) {
	tests := []struct {
		name       string
		steps      []Step
		wantErr    bool
		errContain string
	}{
		{
			name:    "valid DAG",
			steps:   []Step{{ID: "A"}, {ID: "B", DependsOn: []string{"A"}}},
			wantErr: false,
		},
		{
			name:       "orphan dependency",
			steps:      []Step{{ID: "A"}, {ID: "B", DependsOn: []string{"X"}}},
			wantErr:    true,
			errContain: "non-existent step",
		},
		{
			name:    "empty steps",
			steps:   nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDAG(tt.steps)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContain)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
