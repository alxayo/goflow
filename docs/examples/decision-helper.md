# Decision Helper Workflow

An interactive multi-perspective workflow for complex decision-making.

---

## Overview

This example demonstrates:

- **Multiple perspectives** — Different expert viewpoints on the same question
- **Interactive mode** — User input during workflow execution
- **Synthesis** — Combining different perspectives into actionable advice
- **Configurable depth** — Adjusting analysis thoroughness

---

## The Workflow

```yaml title="examples/decision-helper/decision-helper.yaml"
name: "decision-helper"
description: "Multi-perspective analysis for complex decisions"

inputs:
  question:
    description: "The decision you need help with"
    # No default — must be provided
  context:
    description: "Additional context about your situation"
    default: ""
  depth:
    description: "Analysis depth: quick, normal, thorough"
    default: "normal"

config:
  interactive: true  # Enable user interaction

agents:
  interviewer:
    file: "./agents/interviewer.agent.md"
  
  advocate:
    file: "./agents/advocate.agent.md"
  
  critic:
    file: "./agents/critic.agent.md"
  
  synthesizer:
    file: "./agents/synthesizer.agent.md"

steps:
  # Step 1: Understand the decision
  - id: clarify
    agent: interviewer
    prompt: |
      Help me understand this decision:
      
      Question: {{inputs.question}}
      
      Context provided: {{inputs.context}}
      
      Ask 2-3 clarifying questions to understand:
      - What are the main options?
      - What constraints exist?
      - What outcome are they hoping for?

  # Step 2: Gather perspectives (parallel)
  - id: pros-analysis
    agent: advocate
    prompt: |
      Analyze the positive aspects and opportunities:
      
      Decision: {{inputs.question}}
      
      Clarification: {{steps.clarify.output}}
      
      Depth: {{inputs.depth}}
      
      Consider:
      - Benefits of each option
      - Opportunities created
      - Best-case scenarios
      - Why each option might be the right choice
    depends_on: [clarify]

  - id: cons-analysis
    agent: critic
    prompt: |
      Analyze the risks and downsides:
      
      Decision: {{inputs.question}}
      
      Clarification: {{steps.clarify.output}}
      
      Depth: {{inputs.depth}}
      
      Consider:
      - Risks of each option
      - Potential downsides
      - Worst-case scenarios
      - What could go wrong
    depends_on: [clarify]

  # Step 3: Synthesize perspectives
  - id: synthesis
    agent: synthesizer
    prompt: |
      Create a balanced decision framework:
      
      ## The Decision
      {{inputs.question}}
      
      ## Context
      {{steps.clarify.output}}
      
      ## Positive Analysis
      {{steps.pros-analysis.output}}
      
      ## Risk Analysis
      {{steps.cons-analysis.output}}
      
      Provide:
      1. Summary of key trade-offs
      2. Decision matrix (if applicable)
      3. Recommended approach with rationale
      4. Key factors to monitor
      5. Reversibility assessment
    depends_on: [pros-analysis, cons-analysis]

output:
  steps: [synthesis]
  format: markdown
```

---

## Execution Pattern

```
clarify ─┬─→ pros-analysis ──┬─→ synthesis
         │                   │
         └─→ cons-analysis ──┘
             (parallel)
```

---

## Supporting Agent Files

### interviewer.agent.md

```markdown
---
name: interviewer
description: Expert at understanding complex decisions
---

# Decision Interviewer

You help people articulate their decisions clearly.

Your role:
- Ask clarifying questions (but limit to 2-3 key ones)
- Identify the core decision points
- Understand constraints and preferences
- Never judge or steer toward a conclusion

Be warm, curious, and concise.
```

### advocate.agent.md

```markdown
---
name: advocate
description: Explores positive outcomes and opportunities
---

# Decision Advocate

You see the positive potential in each option.

Your role:
- Identify benefits and opportunities  
- Describe best-case scenarios
- Find reasons why each option could work
- Be optimistic but not unrealistic

Present the upside fairly for ALL options, not just the "obvious" choice.
```

### critic.agent.md

```markdown
---
name: critic
description: Identifies risks and potential problems
---

# Decision Critic

You anticipate what could go wrong.

Your role:
- Identify risks and downsides
- Describe worst-case scenarios
- Find potential problems with each option
- Be thorough but not alarmist

Apply scrutiny fairly to ALL options, not just the "risky" ones.
```

### synthesizer.agent.md

```markdown
---
name: synthesizer
description: Combines perspectives into actionable guidance
---

# Decision Synthesizer

You help people make decisions with balanced information.

Your role:
- Combine different perspectives fairly
- Create clear trade-off comparisons
- Provide frameworks, not just opinions
- Respect that the human makes the final call

Output structure:
1. Key trade-offs summary
2. Decision matrix or framework
3. Recommendation with clear rationale
4. Things to monitor after deciding
```

---

## Running the Example

### Interactive Mode

```bash
goflow run \
  --workflow examples/decision-helper/decision-helper.yaml \
  --inputs question='Should I accept a job offer that pays more but requires relocation?' \
  --interactive \
  --verbose
```

### With Context

```bash
goflow run \
  --workflow examples/decision-helper/decision-helper.yaml \
  --inputs question='Should I invest in upgrading our database technology?' \
  --inputs context='We are a 10-person startup with limited budget but growing fast' \
  --inputs depth='thorough' \
  --verbose
```

### Mock Mode (Structure Test)

```bash
goflow run \
  --workflow examples/decision-helper/decision-helper.yaml \
  --inputs question='Test decision' \
  --mock \
  --verbose
```

---

## Sample Output

```markdown
# Decision Framework: Job Offer with Relocation

## Summary of Key Trade-offs

| Factor | Accept | Decline |
|--------|--------|---------|
| Income | +30% increase | Current stable |
| Career | New opportunities | Known trajectory |
| Personal | Major life change | Comfort zone |
| Risk | Higher uncertainty | Lower growth |

## Analysis

### Financial
The 30% raise is significant, but relocation costs 
(moving, housing adjustment, temporary expenses) typically 
consume 3-6 months of the salary difference.

### Career
New role offers exposure to X, Y, Z which aligns with 
stated career goals. However, current role has clear 
promotion path in 12-18 months.

### Personal
Relocation impacts relationships, routines, and support systems.
Consider: Can important relationships survive distance?

## Recommendation

**Lean toward accepting** if career growth is your top priority 
and you have low local obligations.

**Lean toward declining** if stability and relationships are 
more important right now.

## Key Factors to Monitor

If accepting:
- Network building in new location
- Cost of living adjustments
- 6-month happiness check-in

If declining:
- Ensure you don't resent the decision
- Track alternative growth opportunities
- Revisit in 12 months

## Reversibility

**Partially reversible.** You can move back, but:
- Time invested can't be recovered
- Some relationships may not fully restore
- Professional reputation changes may persist
```

---

## Variations

### Add Domain Expert

```yaml
agents:
  domain-expert:
    inline:
      prompt: "You are an expert in {{inputs.domain}}..."

steps:
  - id: domain-analysis
    agent: domain-expert
    depends_on: [clarify]
```

### Add Second Opinion

```yaml
- id: second-opinion
  agent: synthesizer
  prompt: "Review the synthesis and provide a counter-perspective if warranted..."
  depends_on: [synthesis]
  condition:
    step: synthesis
    contains: "UNCERTAIN"
```

---

## See Also

- [Tutorial: Conditions](../tutorial/conditions.md) — Conditional execution
- [Reference: Interactive Mode](../reference/cli.md#interactive-mode) — Interactive features
