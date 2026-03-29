# Global News Trends

Workflow file: `examples/global-news-trends/global-news-trends.yaml`

## What it demonstrates

- High fan-out across multiple source scanners
- Fan-in aggregation into a global trend analyzer
- Large workflow coordination with reusable agent files

## Run

```bash
./goflow run \
  --workflow examples/global-news-trends/global-news-trends.yaml \
  --mock \
  --verbose
```

## Real use case

This is a template for research workflows where many independent collectors feed a central synthesis stage.
