package workflow

import (
	"strings"
	"testing"
)

func TestResolveTemplate(t *testing.T) {
	tests := []struct {
		name      string
		prompt    string
		results   map[string]string
		inputs    map[string]string
		want      string
		wantError bool
		errSubstr string
	}{
		{
			name:    "no templates passthrough",
			prompt:  "Hello, world!",
			results: map[string]string{},
			inputs:  map[string]string{},
			want:    "Hello, world!",
		},
		{
			name:    "single step ref",
			prompt:  "Review: {{steps.analyze.output}}",
			results: map[string]string{"analyze": "found 3 issues"},
			inputs:  map[string]string{},
			want:    "Review: found 3 issues",
		},
		{
			name:    "multiple step refs",
			prompt:  "Security: {{steps.sec.output}}\nPerf: {{steps.perf.output}}",
			results: map[string]string{"sec": "no vulns", "perf": "fast"},
			inputs:  map[string]string{},
			want:    "Security: no vulns\nPerf: fast",
		},
		{
			name:    "input variable",
			prompt:  "Analyze {{inputs.files}}",
			results: map[string]string{},
			inputs:  map[string]string{"files": "src/**/*.go"},
			want:    "Analyze src/**/*.go",
		},
		{
			name:    "mixed steps and inputs",
			prompt:  "Review {{inputs.branch}}: {{steps.analyze.output}}",
			results: map[string]string{"analyze": "looks good"},
			inputs:  map[string]string{"branch": "main"},
			want:    "Review main: looks good",
		},
		{
			name:      "unknown step ref",
			prompt:    "Result: {{steps.missing.output}}",
			results:   map[string]string{},
			inputs:    map[string]string{},
			wantError: true,
			errSubstr: "missing",
		},
		{
			name:      "unknown input ref",
			prompt:    "Files: {{inputs.unknown_var}}",
			results:   map[string]string{},
			inputs:    map[string]string{},
			wantError: true,
			errSubstr: "unknown_var",
		},
		{
			name:    "empty result for known step",
			prompt:  "Output: [{{steps.empty.output}}]",
			results: map[string]string{"empty": ""},
			inputs:  map[string]string{},
			want:    "Output: []",
		},
		{
			name:    "non-matching curly braces passthrough",
			prompt:  "Code: if x {{ do something }} end",
			results: map[string]string{},
			inputs:  map[string]string{},
			want:    "Code: if x {{ do something }} end",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTemplate(tt.prompt, tt.results, tt.inputs)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractTemplateRefs(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		want   []string
	}{
		{"no refs", "plain text", nil},
		{"single ref", "{{steps.analyze.output}}", []string{"analyze"}},
		{"multiple refs", "{{steps.a.output}} and {{steps.b.output}}", []string{"a", "b"}},
		{"hyphenated id", "{{steps.code-review.output}}", []string{"code-review"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTemplateRefs(tt.prompt)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractInputRefs(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		want   []string
	}{
		{"no refs", "plain text", nil},
		{"single ref", "{{inputs.files}}", []string{"files"}},
		{"multiple refs", "{{inputs.a}} {{inputs.b}}", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractInputRefs(tt.prompt)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		cfg           *TruncateConfig
		wantTruncated bool
		wantContains  string // substring the result must contain (suffix check)
		wantLen       int    // if >0, check rune-length of result prefix before suffix
	}{
		{
			name:          "nil config passthrough",
			output:        "hello world",
			cfg:           nil,
			wantTruncated: false,
		},
		{
			name:          "unknown strategy passthrough",
			output:        "hello world",
			cfg:           &TruncateConfig{Strategy: "bogus", Limit: 5},
			wantTruncated: false,
		},
		{
			name:          "empty output no-op",
			output:        "",
			cfg:           &TruncateConfig{Strategy: "chars", Limit: 10},
			wantTruncated: false,
		},
		{
			name:          "chars under limit",
			output:        "short",
			cfg:           &TruncateConfig{Strategy: "chars", Limit: 100},
			wantTruncated: false,
		},
		{
			name:          "chars exactly at limit",
			output:        "12345",
			cfg:           &TruncateConfig{Strategy: "chars", Limit: 5},
			wantTruncated: false,
		},
		{
			name:          "chars over limit",
			output:        "abcdefghij",
			cfg:           &TruncateConfig{Strategy: "chars", Limit: 5},
			wantTruncated: true,
			wantContains:  "truncated: 10 chars total, showing first 5",
		},
		{
			name:          "lines under limit",
			output:        "line1\nline2",
			cfg:           &TruncateConfig{Strategy: "lines", Limit: 5},
			wantTruncated: false,
		},
		{
			name:          "lines exactly at limit",
			output:        "a\nb\nc",
			cfg:           &TruncateConfig{Strategy: "lines", Limit: 3},
			wantTruncated: false,
		},
		{
			name:          "lines over limit",
			output:        "a\nb\nc\nd\ne",
			cfg:           &TruncateConfig{Strategy: "lines", Limit: 2},
			wantTruncated: true,
			wantContains:  "truncated: 5 lines total, showing first 2",
		},
		{
			name:          "tokens uses 4x multiplier under limit",
			output:        "short",
			cfg:           &TruncateConfig{Strategy: "tokens", Limit: 10},
			wantTruncated: false,
		},
		{
			name:          "tokens over limit",
			output:        strings.Repeat("x", 100),
			cfg:           &TruncateConfig{Strategy: "tokens", Limit: 10}, // 10*4=40 char limit
			wantTruncated: true,
			wantContains:  "truncated: 100 chars total, showing first 40",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, truncated := TruncateOutput(tt.output, tt.cfg)
			if truncated != tt.wantTruncated {
				t.Fatalf("truncated=%v, want %v", truncated, tt.wantTruncated)
			}
			if !tt.wantTruncated {
				if got != tt.output {
					t.Errorf("expected passthrough, got %q", got)
				}
				return
			}
			if tt.wantContains != "" && !strings.Contains(got, tt.wantContains) {
				t.Errorf("result %q does not contain %q", got, tt.wantContains)
			}
		})
	}
}
