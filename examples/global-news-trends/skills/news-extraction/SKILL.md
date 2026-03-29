---
name: news-extraction
description: >
  Domain knowledge for extracting headlines from major international news outlets.
  Covers which tool to use per site, URL patterns, timestamp formats, and filtering rules.
applyTo: "**"
---

# News Extraction Skill

Reference knowledge for efficiently extracting headlines from 7 international news outlets.

## Outlet Quick Reference

| Outlet | Homepage URL | URL Date Pattern | Timestamp Format |
|--------|-------------|-----------------|-----------------|
| BBC News | `https://www.bbc.com/news` | `/articles/`, `/news/` | "X hrs ago", "LIVE" |
| NYTimes | `https://www.nytimes.com` | `/2026/03/28/` | "March 28, 2026, 4:07 a.m. ET" |
| Guardian | `https://www.theguardian.com` | `/2026/mar/28/` | Date in URL (no timestamps on page) |
| Yahoo News | `https://news.yahoo.com` | `/articles/` (no dates) | No timestamps — all homepage content is recent |
| Al Jazeera | `https://www.aljazeera.com` | `/2026/3/28/` (no zero-pad) | "Published X minutes ago", "BREAKING" |
| DW News | `https://www.dw.com/en` | `/a-NNNNNNN`, `/live-NNNNNNN` | No timestamps on homepage |
| Euronews | `https://www.euronews.com` | `/2026/03/28/` | ISO 8601 in `time` elements |

## Filtering for Last 24 Hours

- **Relative timestamps:** "X mins ago", "X hrs ago" → always within 24h
- **"1 day ago"** → include (within the 24h window)
- **"LIVE"** or **"BREAKING"** → always include
- **Date in URL:** Compare with today's date and yesterday
- **No timestamp:** Include if on homepage (homepages show current content)

## Content to Exclude

- Navigation links, footer links, section headers
- Subscription/paywall prompts, newsletter signups
- Ads, sponsored content, shopping links
- Evergreen content (book lists, "best of" compilations with old dates)
- Video bulletin teasers without unique news content

## Editorial Perspectives

Understanding outlet perspectives helps with trend analysis:

| Outlet | Based In | Perspective |
|--------|----------|------------|
| BBC | UK | British public broadcaster, balanced, comprehensive world coverage |
| NYTimes | US | US establishment, center-left, deep investigations |
| Guardian | UK | Progressive, strong on climate/social issues |
| Yahoo News | US | Aggregator (AP, Reuters, etc.), breadth across sources |
| Al Jazeera | Qatar | Global South perspective, deep Middle East/Africa/Asia |
| DW | Germany | German public broadcaster, EU policy, development stories |
| Euronews | France | Pan-European, EU regulations, trade, European affairs |
