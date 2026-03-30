# Troubleshooting

Common issues and how to resolve them.

---

## Workflow Validation Errors

### "unknown agent: X"

**Problem:** A step references an agent that isn't defined.

```
Error: unknown agent 'security-reviewer' referenced in step 'scan'
```

**Solution:** Check spelling and define the agent:

```yaml
agents:
  security-reviewer:  # Must match exactly
    inline:
      description: "..."
      prompt: "..."

steps:
  - id: scan
    agent: security-reviewer  # Must match agents section
```

---

### "circular dependency detected"

**Problem:** Steps depend on each other in a loop.

```
Error: circular dependency detected: step-a -> step-b -> step-a
```

**Solution:** Review your `depends_on` chains and break the cycle:

```yaml
# ✗ Bad: Circular
- id: step-a
  depends_on: [step-b]
- id: step-b
  depends_on: [step-a]

# ✓ Good: Linear
- id: step-a
- id: step-b
  depends_on: [step-a]
```

---

### "duplicate step id: X"

**Problem:** Two steps have the same ID.

```
Error: duplicate step id 'analyze'
```

**Solution:** Give each step a unique ID:

```yaml
steps:
  - id: analyze-security  # Unique
    ...
  - id: analyze-performance  # Unique
    ...
```

---

### "missing required input: X"

**Problem:** An input without a default wasn't provided.

```
Error: missing required input: files
```

**Solution:** Either provide the input or add a default:

```bash
# Option 1: Provide at runtime
goflow run --workflow example.yaml --inputs files='src/*.go'
```

```yaml
# Option 2: Add default
inputs:
  files:
    description: "Files to analyze"
    default: "*.go"  # Now optional
```

---

## Runtime Errors

### "step X failed: context deadline exceeded"

**Problem:** A step exceeded its configured timeout limit.

!!! note "Event-Based Completion"
    goflow uses event-based session monitoring by default — sessions complete naturally when the LLM finishes. You only see this error if you explicitly set a `timeout` on the step.

**Solutions:**

1. **Remove the timeout** — If you don't need a strict time limit, remove the `timeout` field and let the session complete naturally

2. **Increase the timeout** — If you need a safety limit, increase it:
   ```yaml
   steps:
     - id: long-analysis
       agent: analyzer
       prompt: "..."
       timeout: "10m"  # More time for complex tasks
   ```

3. **Use --verbose to monitor progress** — See what the agent is doing:
   ```bash
   goflow run --workflow my.yaml --verbose
   ```

4. **Simplify the prompt** — Complex prompts may cause the agent to spin

5. **Split into smaller steps** — Break down the task for better visibility

---

### "agent file not found: X"

**Problem:** Can't find the referenced `.agent.md` file.

```
Error: agent file not found: ./agents/reviewer.agent.md
```

**Solutions:**

1. **Check the path** — Paths are relative to the workflow file:
   ```
   workflows/
     my-workflow.yaml
   agents/
     reviewer.agent.md
   
   # In my-workflow.yaml:
   file: "../agents/reviewer.agent.md"
   ```

2. **Check file extension** — Must be `.agent.md`

3. **Use absolute path** — For debugging:
   ```yaml
   file: "/full/path/to/reviewer.agent.md"
   ```

---

### "template error: step X not found"

**Problem:** Referencing a step that doesn't exist.

```
Error: template error: step 'analize' not found (did you mean 'analyze'?)
```

**Solution:** Fix the typo in your template:

```yaml
# ✗ Bad
prompt: "Use {{steps.analize.output}}"

# ✓ Good
prompt: "Use {{steps.analyze.output}}"
```

---

### "condition references unknown step"

**Problem:** A condition references a non-existent step.

```
Error: condition references unknown step: 'check'
```

**Solution:** Verify the step ID in your condition:

```yaml
- id: review
  condition:
    step: initial-check  # Must match an actual step ID
    contains: "APPROVE"
```

---

## Mock Mode Issues

### Mock mode shows "mock output" but real mode fails

**Problem:** Workflow works in mock mode but fails with real AI calls.

**Possible causes:**

1. **Copilot CLI not installed**
   ```bash
   which copilot
   # If empty, install Copilot CLI
   # The SDK executor manages the CLI automatically, but the binary must be on PATH
   ```

2. **Not authenticated**
   ```bash
   copilot auth status
   ```

3. **Model not available**
   — Check if the specified model is accessible to your account

4. **Try the CLI fallback**
   — If the SDK executor has issues, run with `--cli` to use the legacy subprocess executor:
   ```bash
   goflow run --workflow my-workflow.yaml --cli
   ```

---

## Agent Issues

### Agent tools not working

**Problem:** Agent can't use expected tools.

**Solutions:**

1. **Check tool is listed:**
   ```yaml
   agents:
     reviewer:
       inline:
         tools:
           - grep
           - read_file  # Must be listed
   ```

2. **Use correct tool names:**
   ```yaml
   # ✓ Correct
   tools: [grep, semantic_search, read_file]
   
   # ✗ Wrong
   tools: [search, open_file, terminal]
   ```

---

### Agent seems to ignore instructions

**Problem:** Agent doesn't follow the system prompt.

**Solutions:**

1. **Make instructions clearer:**
   ```yaml
   prompt: |
     IMPORTANT: You MUST do X.
     NEVER do Y.
     Always format output as [FORMAT].
   ```

2. **Check prompt isn't too long** — Very long prompts may have key instructions ignored

3. **Use stronger directives:**
   ```yaml
   prompt: |
     CRITICAL RULES (follow exactly):
     1. Always cite line numbers
     2. Use severity levels: CRITICAL, HIGH, MEDIUM, LOW
     3. Never use the phrase "I think"
   ```

---

## Audit Trail Issues

### Audit directory not created

**Problem:** No `.workflow-runs/` directory appears.

**Solutions:**

1. **Check write permissions:**
   ```bash
   touch .workflow-runs/test
   ```

2. **Specify explicit path:**
   ```yaml
   config:
     audit_dir: "/tmp/goflow-runs"
   ```

---

### Old runs not cleaned up

**Problem:** Audit directory grows indefinitely.

**Solution:** Configure retention:

```yaml
config:
  audit_retention: 10  # Keep only last 10 runs
```

---

## Template Issues

### Template not replaced

**Problem:** `{{inputs.X}}` or `{{steps.Y.output}}` appears literally in output.

**Solutions:**

1. **Check syntax** — No spaces inside braces:
   ```yaml
   # ✓ Good
   {{inputs.files}}
   
   # ✗ Bad
   {{ inputs.files }}
   ```

2. **Check field name exists:**
   ```yaml
   inputs:
     files:  # This name
       ...
   
   prompt: "{{inputs.files}}"  # Must match
   ```

---

### Output truncated unexpectedly

**Problem:** Step output is shorter than expected.

**Solution:** Adjust truncation settings:

```yaml
config:
  truncate:
    strategy: "lines"
    limit: 500  # Increase limit
```

Or disable for specific output:

```yaml
output:
  steps: [summary]
  truncate:
    strategy: "chars"
    limit: 999999  # Effectively unlimited
```

---

## Getting More Help

### 1. Enable Verbose Mode

```bash
goflow run --workflow example.yaml --verbose
```

Shows step-by-step progress and timing.

### 2. Check the Audit Trail

Every run saves the exact prompts and outputs:

```bash
cat .workflow-runs/*/steps/*/prompt.md   # What was sent
cat .workflow-runs/*/steps/*/output.md   # What was received
```

### 3. Run Single Steps

Isolate problem steps:

```bash
goflow run --workflow example.yaml --step problematic-step --mock
```

### 4. Validate First

Check YAML structure before running:

```bash
goflow validate --workflow example.yaml
```

---

## Still Stuck?

1. **Search existing issues** on GitHub
2. **Create a minimal reproducible example** 
3. **Open an issue** with:
   - Your workflow file (sanitized)
   - The error message
   - `goflow version` output
   - Steps to reproduce
