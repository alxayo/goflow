# Examples

goflow includes ready-to-run workflows that demonstrate common orchestration patterns.

## Available examples

- **Simple Sequential**: baseline single-lane review pipeline
- **Security Scan**: multi-tool and multi-agent security analysis
- **Guided Code Review**: scoped deep-dives with multiple reviewers
- **Decision Helper**: structured multi-perspective decision support
- **Global News Trends**: large fan-out and aggregation workflow

## How to run any example

```bash
./goflow run --workflow <example-path.yaml> --mock --verbose
```

Then switch to real execution by removing `--mock`.

## Example selection guidance

- New users: start with `examples/simple-sequential.yaml`
- Security and compliance: use `examples/security-scan/security-scan.yaml`
- Rich reviewer coordination: use `examples/guided-code-review/guided-code-review.yaml`
- Decision making workflows: use `examples/decision-helper/decision-helper.yaml`
- High parallelism use cases: use `examples/global-news-trends/global-news-trends.yaml`
