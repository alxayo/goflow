# Global News Trends Workflow

A self-contained workflow that scans 7 international news outlets in parallel, extracts headlines from the last 24 hours, and identifies globally trending topics by cross-referencing coverage across all sources.

## How It Works

```
┌─────────────┐
│  Start       │
└──────┬──────┘
       │
       ▼
┌──────────────────────────────────────────────────────────────┐
│  Fan-Out: 7 parallel headline scans                          │
│                                                              │
│  scan-bbc ─────────┐                                         │
│  scan-nytimes ─────┤                                         │
│  scan-guardian ────┤                                         │
│  scan-yahoo ───────┤  (all run concurrently)                 │
│  scan-aljazeera ───┤                                         │
│  scan-dw ──────────┤                                         │
│  scan-euronews ────┘                                         │
└────────────────────────────┬─────────────────────────────────┘
                             │
                             ▼
                  ┌─────────────────────┐
                  │  Fan-In:            │
                  │  analyze-trends     │
                  │  (waits for all 7)  │
                  └──────────┬──────────┘
                             │
                             ▼
                   ┌────────────────┐
                   │  Global        │
                   │  Trending      │
                   │  Topics Report │
                   └────────────────┘
```

## Folder Structure

This workflow is **self-contained** — all agents, skills, and the workflow YAML live in this folder. goflow resolves all paths relative to the workflow file location.

```
examples/global-news-trends/
├── global-news-trends.yaml              # Workflow definition
├── README.md                            # This file
├── agents/
│   ├── bbc-scanner.agent.md             # BBC News headline extractor
│   ├── nytimes-scanner.agent.md         # NYTimes headline extractor
│   ├── guardian-scanner.agent.md        # Guardian headline extractor
│   ├── yahoo-scanner.agent.md           # Yahoo News headline extractor
│   ├── aljazeera-scanner.agent.md       # Al Jazeera headline extractor
│   ├── dw-scanner.agent.md              # DW News headline extractor
│   ├── euronews-scanner.agent.md        # Euronews headline extractor
│   └── global-trend-analyzer.agent.md   # Trend analysis aggregator
└── skills/
    └── news-extraction/
        └── SKILL.md                     # Outlet-specific parsing knowledge
```

## News Sources

| # | Outlet | Region | Perspective |
|---|--------|--------|------------|
| 1 | BBC News | UK | British public broadcaster, balanced |
| 2 | New York Times | US | US establishment, deep investigations |
| 3 | The Guardian | UK | Progressive, social/climate focus |
| 4 | Yahoo News | US | Aggregator (AP, Reuters, others) |
| 5 | Al Jazeera | Qatar | Global South, Middle East depth |
| 6 | DW News | Germany | German/EU policy, development |
| 7 | Euronews | EU | Pan-European, EU regulations |

## Running the Workflow

### Default (all 7 outlets)

```bash
# From the repo root
go build -o goflow ./cmd/workflow-runner/main.go

./goflow run \
  --workflow examples/global-news-trends/global-news-trends.yaml \
  --verbose
```

### With custom news sites

```bash
./goflow run \
  --workflow examples/global-news-trends/global-news-trends.yaml \
  --inputs site_bbc=https://www.bbc.com/news \
  --inputs site_nytimes=https://www.nytimes.com \
  --inputs site_guardian=https://www.theguardian.com/us \
  --inputs site_yahoo=https://news.yahoo.com \
  --inputs site_aljazeera=https://www.aljazeera.com \
  --inputs site_dw=https://www.dw.com/en \
  --inputs site_euronews=https://www.euronews.com
```

### Dry run with mock executor

```bash
./goflow run \
  --workflow examples/global-news-trends/global-news-trends.yaml \
  --mock --verbose
```

## Output

The workflow produces a Markdown report with four tiers:

- **Top Trending (5-7 sources)** — Stories covered by nearly every outlet
- **Trending (3-4 sources)** — Stories with significant cross-source presence
- **Notable (2 sources)** — Stories confirmed by at least two outlets
- **Regional / Single-Source** — Stories unique to a specific outlet, grouped by region

Each trending topic includes:
- Which sources covered it
- A 2-3 sentence synthesis across sources
- Divergent framing notes (where outlets frame the same event differently)

## Audit Trail

```
.workflow-runs/2026-03-28T10-00-00_global-news-trends/
├── workflow.meta.json
├── workflow.yaml
├── final_output.md
└── steps/
    ├── 00_scan-bbc/
    ├── 00_scan-nytimes/
    ├── 00_scan-guardian/
    ├── 00_scan-yahoo/
    ├── 00_scan-aljazeera/
    ├── 00_scan-dw/
    ├── 00_scan-euronews/
    └── 01_analyze-trends/
```

## Differences from `trending-news` Workflow

| Feature | `trending-news` | `global-news-trends` |
|---------|-----------------|---------------------|
| Sources | 5 (BBC, CNN, Reuters, Google News, AP) | 7 (BBC, NYT, Guardian, Yahoo, Al Jazeera, DW, Euronews) |
| Agents | 1 generic scanner + 1 analyzer | 7 specialized scanners + 1 analyzer |
| Self-contained | No (agents in repo root) | Yes (everything in one folder) |
| Perspective coverage | Mostly US/UK | US, UK, EU, Middle East |
| Divergent framing | Not tracked | Noted when outlets frame stories differently |
| Regional grouping | No | Single-source stories grouped by region |
