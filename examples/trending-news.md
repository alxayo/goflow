# Trending News Workflow

A workflow that scans the top 5 news sites in parallel, extracts headlines, and identifies trending topics by cross-referencing coverage across all sources.

## How It Works

```
┌─────────────┐
│  Start       │
└──────┬──────┘
       │
       ▼
┌──────────────────────────────────────────────────────┐
│  Fan-Out: 5 parallel headline scans                  │
│                                                      │
│  scan-bbc ─────┐                                     │
│  scan-cnn ─────┤                                     │
│  scan-reuters ─┤  (all run concurrently)             │
│  scan-google ──┤                                     │
│  scan-ap ──────┘                                     │
└──────────────────────────┬───────────────────────────┘
                           │
                           ▼
                ┌─────────────────────┐
                │  Fan-In:            │
                │  analyze-trends     │
                │  (waits for all 5)  │
                └──────────┬──────────┘
                           │
                           ▼
                 ┌────────────────┐
                 │  Trending      │
                 │  Topics Report │
                 └────────────────┘
```

### Agents

| Agent | File | Role |
|-------|------|------|
| `news-scanner` | `agents/news-scanner.agent.md` | Fetches a news site via `fetch_webpage` and extracts a numbered list of headlines |
| `trend-analyzer` | `agents/trend-analyzer.agent.md` | Receives all headlines, clusters by topic, counts cross-source coverage, ranks by trending intensity |

### Steps

1. **`scan-bbc`** — Fetches headlines from BBC News
2. **`scan-cnn`** — Fetches headlines from CNN
3. **`scan-reuters`** — Fetches headlines from Reuters
4. **`scan-google-news`** — Fetches headlines from Google News
5. **`scan-ap`** — Fetches headlines from AP News
6. **`analyze-trends`** — Aggregates all headlines and produces a trending topics report grouped by coverage breadth

Steps 1–5 run in **parallel** (no `depends_on` between them). Step 6 **fans in**, waiting for all 5 scans to complete before executing.

## Running the Workflow

### With default news sites (BBC, CNN, Reuters, Google News, AP)

```bash
go build -o workflow-runner ./cmd/workflow-runner/main.go

./workflow-runner run --workflow examples/trending-news.yaml --verbose
```

### With custom news sites

```bash
./workflow-runner run --workflow examples/trending-news.yaml \
  --inputs site_1=https://www.bbc.com/news \
  --inputs site_2=https://www.nytimes.com \
  --inputs site_3=https://www.theguardian.com \
  --inputs site_4=https://news.yahoo.com \
  --inputs site_5=https://www.aljazeera.com
```

### Dry run with mock executor (no API calls)

```bash
./workflow-runner run --workflow examples/trending-news.yaml --mock --verbose
```

## Output

The workflow produces a Markdown report with three tiers:

- **Top Trending (4–5 sources)** — Stories covered by nearly every outlet
- **Trending (2–3 sources)** — Stories with significant cross-source presence
- **Single-Source Stories** — Notable headlines found on only one site

Each trending topic includes:
- Which sources covered it
- A 2–3 sentence synthesis across all sources

## Audit Trail

Each run creates a timestamped directory under `.workflow-runs/`:

```
.workflow-runs/2026-03-26T16-28-26_trending-news/
├── workflow.meta.json      # Run metadata (timing, status)
├── workflow.yaml           # Snapshot of workflow used
├── final_output.md         # The trending topics report
└── steps/
    ├── 00_scan-bbc/        # Level 0: parallel scans
    ├── 00_scan-cnn/
    ├── 00_scan-reuters/
    ├── 00_scan-google-news/
    ├── 00_scan-ap/
    └── 01_analyze-trends/  # Level 1: fan-in aggregation
```

## Customization Ideas

- **Change sources:** Override any `site_N` input to point at a different news outlet.
- **Add more sites:** Add new scan steps and inputs in the YAML, then add them to `analyze-trends`'s `depends_on` list and prompt template.
- **Filter by topic:** Add a post-analysis step with a condition (e.g., `contains: "technology"`) to filter the report to a specific domain.
- **Schedule it:** Run via cron or CI to produce daily/hourly trend snapshots.
