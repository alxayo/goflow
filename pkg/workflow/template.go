// template.go resolves {{steps.X.output}} and {{inputs.Y}} placeholders in
// prompt strings. It supports step output references and workflow input
// variable substitution. It also provides output truncation strategies
// (chars, lines, tokens) to prevent context window overflow.
package workflow

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	stepOutputRe = regexp.MustCompile(`\{\{steps\.([a-zA-Z0-9_-]+)\.output\}\}`)
	inputRe      = regexp.MustCompile(`\{\{inputs\.([a-zA-Z0-9_-]+)\}\}`)
)

// ResolveTemplate substitutes {{steps.X.output}} and {{inputs.Y}} placeholders
// in a prompt string. Returns an error if a referenced step has no result yet
// or an input variable is not defined. Step refs are processed first, then
// input refs.
func ResolveTemplate(prompt string, results map[string]string, inputs map[string]string) (string, error) {
	// Process step output references first.
	var resolveErr error
	resolved := stepOutputRe.ReplaceAllStringFunc(prompt, func(match string) string {
		if resolveErr != nil {
			return match
		}
		subs := stepOutputRe.FindStringSubmatch(match)
		stepID := subs[1]
		val, ok := results[stepID]
		if !ok {
			resolveErr = fmt.Errorf("template references unknown step %q", stepID)
			return match
		}
		return val
	})
	if resolveErr != nil {
		return "", resolveErr
	}

	// Process input references.
	resolved = inputRe.ReplaceAllStringFunc(resolved, func(match string) string {
		if resolveErr != nil {
			return match
		}
		subs := inputRe.FindStringSubmatch(match)
		name := subs[1]
		val, ok := inputs[name]
		if !ok {
			resolveErr = fmt.Errorf("template references unknown input %q", name)
			return match
		}
		return val
	})
	if resolveErr != nil {
		return "", resolveErr
	}

	return resolved, nil
}

// ExtractTemplateRefs returns all step IDs referenced via {{steps.X.output}}.
func ExtractTemplateRefs(prompt string) []string {
	matches := stepOutputRe.FindAllStringSubmatch(prompt, -1)
	refs := make([]string, 0, len(matches))
	for _, m := range matches {
		refs = append(refs, m[1])
	}
	return refs
}

// ExtractInputRefs returns all input names referenced via {{inputs.Y}}.
func ExtractInputRefs(prompt string) []string {
	matches := inputRe.FindAllStringSubmatch(prompt, -1)
	refs := make([]string, 0, len(matches))
	for _, m := range matches {
		refs = append(refs, m[1])
	}
	return refs
}

// TruncateOutput applies the configured truncation strategy to a step's output
// before injecting it into a template. Returns the truncated string and a bool
// indicating whether truncation occurred.
//
// Strategies:
//   - "chars": keep first cfg.Limit characters
//   - "lines": keep first cfg.Limit lines
//   - "tokens": approximate 1 token ≈ 4 chars, keep first cfg.Limit*4 characters
//
// A nil config or unknown strategy returns the output unchanged.
func TruncateOutput(output string, cfg *TruncateConfig) (string, bool) {
	if cfg == nil {
		return output, false
	}

	switch cfg.Strategy {
	case "chars":
		return truncateByChars(output, cfg.Limit)
	case "lines":
		return truncateByLines(output, cfg.Limit)
	case "tokens":
		// Approximate 1 token ≈ 4 characters.
		return truncateByTokens(output, cfg.Limit)
	default:
		return output, false
	}
}

func truncateByChars(output string, limit int) (string, bool) {
	totalChars := len([]rune(output))
	if totalChars <= limit {
		return output, false
	}
	truncated := string([]rune(output)[:limit])
	suffix := fmt.Sprintf("\n\n... [truncated: %d chars total, showing first %d]", totalChars, limit)
	return truncated + suffix, true
}

func truncateByLines(output string, limit int) (string, bool) {
	lines := strings.Split(output, "\n")
	totalLines := len(lines)
	if totalLines <= limit {
		return output, false
	}
	truncated := strings.Join(lines[:limit], "\n")
	suffix := fmt.Sprintf("\n\n... [truncated: %d lines total, showing first %d]", totalLines, limit)
	return truncated + suffix, true
}

func truncateByTokens(output string, limit int) (string, bool) {
	charLimit := limit * 4
	totalChars := len([]rune(output))
	if totalChars <= charLimit {
		return output, false
	}
	truncated := string([]rune(output)[:charLimit])
	suffix := fmt.Sprintf("\n\n... [truncated: %d chars total, showing first %d]", totalChars, charLimit)
	return truncated + suffix, true
}
