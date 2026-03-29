# Tutorial Overview

Welcome to the goflow tutorial! This tutorial teaches you goflow features progressively, building on concepts from previous sections.

---

## Learning Path

Each tutorial builds on the previous one:

| # | Tutorial | What You'll Learn |
|---|----------|-------------------|
| 1 | [Adding Inputs](inputs.md) | Make workflows configurable with runtime parameters |
| 2 | [Multi-Step Pipelines](multi-step.md) | Chain steps and pass data between them |
| 3 | [Parallel Execution](parallel.md) | Run multiple steps simultaneously |
| 4 | [Conditional Logic](conditions.md) | Branch based on step outputs |
| 5 | [Agent Files](agent-files.md) | Organize agents in reusable `.agent.md` files |

---

## Prerequisites

Before starting this tutorial, you should:

- [x] Have goflow installed ([Installation](../getting-started/installation.md))
- [x] Understand the basic workflow structure ([Your First Workflow](../getting-started/first-workflow.md))
- [x] Know how to run workflows with `goflow run`

---

## Tutorial Style

Each tutorial follows this pattern:

1. **Goal** — What we're building
2. **Code** — Complete working examples
3. **Explanation** — Line-by-line breakdown
4. **Try It** — Commands to run
5. **What You Learned** — Key takeaways
6. **Next Steps** — What to explore next

All examples are designed to work in both **mock mode** and **real mode**, so you can follow along without needing Copilot CLI.

---

## Quick Reference

As you learn, these reference pages will be helpful:

- [Workflow YAML Schema](../reference/workflow-schema.md) — Complete field reference
- [Template Variables](../reference/templates.md) — `{{inputs.X}}` and `{{steps.Y.output}}` syntax
- [CLI Reference](../reference/cli.md) — All command-line options

---

## Getting Help

If you get stuck:

1. **Check the error message** — goflow provides detailed validation errors
2. **Use `--verbose`** — See step-by-step progress
3. **Check the audit trail** — `.workflow-runs/` contains the exact prompts and outputs
4. **Visit [Troubleshooting](../troubleshooting.md)** — Common issues and solutions

---

## Let's Begin!

Start with the first tutorial: **[Adding Inputs](inputs.md)**
