package workflow

import (
	"testing"
)

func TestEvaluateCondition(t *testing.T) {
	tests := []struct {
		name      string
		cond      *Condition
		results   map[string]string
		want      bool
		wantError bool
	}{
		{
			name: "contains match",
			cond: &Condition{Step: "decide", Contains: "APPROVE"},
			results: map[string]string{
				"decide": "Result: APPROVE the change",
			},
			want: true,
		},
		{
			name: "contains no match",
			cond: &Condition{Step: "decide", Contains: "APPROVE"},
			results: map[string]string{
				"decide": "Result: REJECT the change",
			},
			want: false,
		},
		{
			name: "not_contains substring present",
			cond: &Condition{Step: "decide", NotContains: "REJECT"},
			results: map[string]string{
				"decide": "Result: REJECT the change",
			},
			want: false,
		},
		{
			name: "not_contains substring absent",
			cond: &Condition{Step: "decide", NotContains: "REJECT"},
			results: map[string]string{
				"decide": "Result: APPROVE the change",
			},
			want: true,
		},
		{
			name: "equals exact match",
			cond: &Condition{Step: "decide", Equals: "APPROVE"},
			results: map[string]string{
				"decide": "APPROVE",
			},
			want: true,
		},
		{
			name: "equals match with surrounding whitespace",
			cond: &Condition{Step: "decide", Equals: "APPROVE"},
			results: map[string]string{
				"decide": "  APPROVE  \n",
			},
			want: true,
		},
		{
			name: "equals mismatch",
			cond: &Condition{Step: "decide", Equals: "APPROVE"},
			results: map[string]string{
				"decide": "REJECT",
			},
			want: false,
		},
		{
			name: "nil condition",
			cond: nil,
			results: map[string]string{
				"decide": "anything",
			},
			want: true,
		},
		{
			name:      "missing step output",
			cond:      &Condition{Step: "nonexistent", Contains: "APPROVE"},
			results:   map[string]string{},
			want:      false,
			wantError: true,
		},
		{
			name: "empty operators in condition",
			cond: &Condition{Step: "decide"},
			results: map[string]string{
				"decide": "anything",
			},
			want: true,
		},
		{
			name: "case sensitivity APPROVE vs approve",
			cond: &Condition{Step: "decide", Contains: "APPROVE"},
			results: map[string]string{
				"decide": "approve",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvaluateCondition(tt.cond, tt.results)
			if (err != nil) != tt.wantError {
				t.Fatalf("EvaluateCondition() error = %v, wantError = %v", err, tt.wantError)
			}
			if tt.wantError && err != nil {
				// Verify error message contains the step name
				if tt.cond != nil && !contains(err.Error(), tt.cond.Step) {
					t.Errorf("error message %q should reference step %q", err.Error(), tt.cond.Step)
				}
				return
			}
			if got != tt.want {
				t.Errorf("EvaluateCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

// contains checks if s contains substr (helper to avoid importing strings in test).
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
