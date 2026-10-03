# Public Retrieval Examples

Optional examples; [SKILL.md](../SKILL.md) owns retrieval order, access stops, and evidence.
A command here is usable only when exposed/installed and permitted; do not install it silently.
The direct boundary probe precedes alternatives; no example authorizes bypassing a block.

## Query Recipes

   - **Factual lookup**: `"exact error message or API name" — use quotes for literal strings`
   - **Comparison**: `"X vs Y" OR "X compared to Y" site:github.com OR site:stackoverflow.com`
   - **Documentation**: `site:docs.example.com feature-name — limit to official docs domain`
   - **Recent (last year)**: add `after:2025-01-01` to GitHub code search or use news/article sources
   - **Code examples**: `site:github.com filename:*.go "function or pattern" — find real usage`
   - **Academic/arXiv**: `site:arxiv.org "topic" — prefer papers from last 2 years`
   - **Breaking changes**: `"changelog" OR "release notes" OR "migration guide" OR "breaking" library-name version`
   - **Known bugs**: `site:github.com/library-owner/library-repo/issues "symptom description"`
   - **Avoid**: single broad terms ("database", "optimization") — too vague to produce useful results.

## Platform Public Routes

| Platform | Method | Example command |
|----------|--------|----------------|
| **Reddit** | `.json` suffix + Mobile UA | `curl -sL -H "User-Agent: Mozilla/5.0 (iPhone; ...)" "https://www.reddit.com/r/{sub}/hot.json?limit=10"` |
| **Hacker News** | Firebase API | `curl -sL "https://hacker-news.firebaseio.com/v0/topstories.json?limitToFirst=10&orderBy=%22%24key%22"` |
| **arXiv** | Atom API | `curl -sL "http://export.arxiv.org/api/query?search_query={query}&start=0&max_results=10"` |
| **GitHub** | `gh` CLI / REST | `gh search repos "{query}" --limit 10 --json name,url,description` |
| **Wikipedia** | REST API | `curl -sL "https://en.wikipedia.org/api/rest_v1/page/summary/{title}"` |
| **Stack Overflow** | SE API v2.3 | `curl -sL "https://api.stackexchange.com/2.3/search?order=desc&sort=relevance&intitle={query}&site=stackoverflow"` |
| **npm / PyPI** | Registry API | `curl -sL "https://registry.npmjs.org/{pkg}"` / `curl -sL "https://pypi.org/pypi/{pkg}/json"` |
| **Wayback Machine** | CDX API | `curl -sL "https://web.archive.org/cdx/search/cdx?url={domain}/*&output=json&limit=10"` |
| **YouTube / 1,858 media sites** | yt-dlp metadata | `yt-dlp --dump-json --skip-download "{URL}" 2>/dev/null` |


## Metadata from an Already-Received Response

When only a blocked page shell is available, structured metadata can still provide titles, summaries, prices, or profile info:

```bash
# OpenGraph tags (title, description, image, URL)
rg -o '<meta property="og:(title|description|image|url)"[^>]*content="[^"]*"' page.html

# JSON-LD structured data (Schema.org — product prices, article bodies, person profiles)
python3 -c "
import re, json
html = open('page.html').read()
for block in re.findall(r'<script type=\"application/ld\+json\">(.*?)</script>', html, re.DOTALL):
    try: print(json.dumps(json.loads(block), indent=2))
    except: pass
"

# Twitter Card metadata
rg -o '<meta name="twitter:(title|description|image|creator)"[^>]*content="[^"]*"' page.html
```

Optional dependencies include `beautifulsoup4`, `feedparser`, and `yt-dlp`. Check availability and authorization first.
