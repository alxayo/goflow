package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	input := flag.String("input", "", "Path to a gh-aw .md file or directory of .md files (required)")
	output := flag.String("output", "./translated/", "Output directory for generated files")
	interactive := flag.Bool("interactive", true, "Set interactive mode on translated workflow steps")
	githubMCP := flag.String("github-mcp-command", "npx -y @modelcontextprotocol/server-github", "Command for the GitHub MCP server")
	dryRun := flag.Bool("dry-run", false, "Print output without writing files")
	verbose := flag.Bool("verbose", false, "Show detailed translation decisions")
	mergeChains := flag.Bool("merge-chains", true, "Auto-detect and merge connected workflows into DAGs")
	model := flag.String("model", "", "Override model for all translated workflows")
	skipImports := flag.Bool("skip-imports", false, "Do not resolve imports: references")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "gh-aw-translate: convert GitHub Agentic Workflows to goflow format\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  gh-aw-translate --input <path> --output <dir> [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *input == "" {
		fmt.Fprintln(os.Stderr, "error: --input is required")
		flag.Usage()
		os.Exit(2)
	}

	info, err := os.Stat(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot access %q: %v\n", *input, err)
		os.Exit(2)
	}

	_ = info
	_ = output
	_ = interactive
	_ = githubMCP
	_ = dryRun
	_ = verbose
	_ = mergeChains
	_ = model
	_ = skipImports

	// TODO(Phase 1): implement translation pipeline
	// 1. Parse input file(s) — pkg/parser
	// 2. Scan and rewrite expressions — pkg/expression
	// 3. Map tools, engine, MCP, safe-outputs — pkg/mapper
	// 4. Detect and merge chains (if directory mode) — pkg/chain
	// 5. Emit goflow YAML, agent files, translation notes — pkg/emitter
	fmt.Println("gh-aw-translate: not yet implemented")
	os.Exit(1)
}
