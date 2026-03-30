package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunArgsVersion(t *testing.T) {
	originalVersion := version
	originalCommit := commit
	originalBuildDate := buildDate
	defer func() {
		version = originalVersion
		commit = originalCommit
		buildDate = originalBuildDate
	}()

	version = "v1.2.3"
	commit = "abc1234"
	buildDate = "2026-03-29T12:00:00Z"

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runArgs([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runArgs returned %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{"goflow v1.2.3", "commit: abc1234", "built: 2026-03-29T12:00:00Z"} {
		if !strings.Contains(output, want) {
			t.Errorf("version output %q does not contain %q", output, want)
		}
	}
}

func TestRunArgsUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runArgs([]string{"bogus"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runArgs returned %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unknown command \"bogus\"") {
		t.Fatalf("stderr = %q, want unknown command message", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: goflow run [options]") {
		t.Fatalf("stderr = %q, want usage text", stderr.String())
	}
}

func TestRunArgsHelpIncludesStreamFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runArgs([]string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runArgs returned %d, want 0", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "--stream") {
		t.Errorf("help output does not contain --stream flag")
	}
	if !strings.Contains(output, "Stream LLM output") {
		t.Errorf("help output does not contain stream description")
	}
}
