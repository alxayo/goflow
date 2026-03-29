// interactive_test.go verifies the IsInteractive resolution logic that
// determines whether a step should allow user input.
//
// Resolution priority (highest to lowest): step.Interactive > config.interactive.
// The CLI --interactive flag is a mechanism gate (wires up the handler) but does
// NOT change the default for steps that have not explicitly opted in.
package workflow

import "testing"

func TestIsInteractive(t *testing.T) {
	tests := []struct {
		name           string
		stepValue      *bool // nil = unset, true/false = explicit
		wfInteractive  bool
		cliInteractive bool
		want           bool
	}{
		// When nothing is set, the step should NOT be interactive.
		{
			name:           "all defaults — not interactive",
			stepValue:      nil,
			wfInteractive:  false,
			cliInteractive: false,
			want:           false,
		},
		// CLI flag alone does NOT enable interactivity for unset steps.
		// The flag only wires up the handler; steps must opt in explicitly.
		{
			name:           "CLI flag alone does not enable unset step",
			stepValue:      nil,
			wfInteractive:  false,
			cliInteractive: true,
			want:           false,
		},
		// Workflow config alone enables interactivity for unset steps.
		{
			name:           "workflow config enables",
			stepValue:      nil,
			wfInteractive:  true,
			cliInteractive: false,
			want:           true,
		},
		// CLI + workflow config: workflow config drives the unset step, CLI is irrelevant here.
		{
			name:           "workflow config enables even with CLI flag",
			stepValue:      nil,
			wfInteractive:  true,
			cliInteractive: true,
			want:           true,
		},
		// Step explicitly set to true overrides everything.
		{
			name:           "step explicitly true overrides all-false",
			stepValue:      BoolPtr(true),
			wfInteractive:  false,
			cliInteractive: false,
			want:           true,
		},
		// Step explicitly set to false overrides workflow config.
		{
			name:           "step explicitly false overrides workflow config",
			stepValue:      BoolPtr(false),
			wfInteractive:  true,
			cliInteractive: false,
			want:           false,
		},
		// Step explicitly set to false overrides CLI flag.
		{
			name:           "step explicitly false overrides CLI flag",
			stepValue:      BoolPtr(false),
			wfInteractive:  false,
			cliInteractive: true,
			want:           false,
		},
		// Step explicitly set to false overrides both.
		{
			name:           "step explicitly false overrides both",
			stepValue:      BoolPtr(false),
			wfInteractive:  true,
			cliInteractive: true,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := Step{
				ID:          "test-step",
				Interactive: tt.stepValue,
			}
			got := IsInteractive(step, tt.wfInteractive, tt.cliInteractive)
			if got != tt.want {
				t.Errorf("IsInteractive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBoolPtr(t *testing.T) {
	truePtr := BoolPtr(true)
	if truePtr == nil || *truePtr != true {
		t.Error("BoolPtr(true) should return pointer to true")
	}

	falsePtr := BoolPtr(false)
	if falsePtr == nil || *falsePtr != false {
		t.Error("BoolPtr(false) should return pointer to false")
	}
}
