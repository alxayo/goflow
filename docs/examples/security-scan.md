# Security Scan

Workflow file: `examples/security-scan/security-scan.yaml`

## What it demonstrates

- Multi-agent security scanning
- Integration with specialized scanners/tools
- Aggregation and remediation planning
- Security-focused audit pipeline

## Run

```bash
./goflow run \
  --workflow examples/security-scan/security-scan.yaml \
  --inputs files='**/*' \
  --mock \
  --verbose
```

## Real use case

Use this to create consistent, repeatable security reports and remediation plans across repositories.
