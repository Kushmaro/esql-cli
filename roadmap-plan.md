# Roadmap & Plan

Status of the `esql` CLI — what exists, what's next, and what full ES|QL support looks like.

---

## Phase 0: Scaffolding [DONE]

The project skeleton is in place. Nothing executes real queries yet.

- [x] Go module initialized with Cobra + Viper
- [x] `main.go` entrypoint
- [x] Root command with global flags (`--host`, `--user`, `--password`, `--api-key`, `--insecure`, `--config`)
- [x] Config cascade: file (`~/.esql.yaml`) → env vars (`ESQL_*`) → CLI flags
- [x] `esql query` subcommand (stub — prints placeholder)
- [x] `esql init` subcommand (stub)
- [x] ES HTTP client with basic auth + API key auth support
- [x] Output formatter with table, JSON, and CSV modes
- [x] Project layout: `cmd/`, `internal/client/`, `internal/config/`, `internal/formatter/`, `internal/repl/`

---

## Phase 1: Core Query Execution [DONE]

Wire the `esql query` command end-to-end so it actually hits Elasticsearch and returns results.

- [x] Connect `cmd/query.go` → `internal/config` → `internal/client` → `internal/formatter`
- [x] POST the user's query string to `POST /_query` on the configured cluster
- [x] Parse the ES|QL columnar response (`columns` + `values`)
- [x] Render results in the chosen format (`--format table|json|csv|tsv`)
- [x] Handle ES error responses gracefully (surface the `root_cause` message)
- [x] Exit code: 0 on success, 1 on query/connection error

### Pipe-friendly output

The tool must behave well in unix pipelines from day one:

- [x] Detect TTY vs pipe (`os.Stdout` is a terminal or not)
- [x] When piped, default to **no headers, no borders** (raw tab-separated) so output is `grep`/`awk`/`cut`-friendly
- [x] `--format` flag overrides auto-detection
- [x] No color codes when piped (only when TTY)
- [x] Support `--no-header` flag to suppress column headers in any format

**Goal:** after Phase 1, this works:

```bash
esql query 'FROM logs-* | WHERE status >= 500 | LIMIT 100' --format csv | cut -d, -f2,5 | sort | uniq -c
esql query 'FROM metrics | STATS avg_cpu = AVG(cpu) BY host' --format json | jq '.[].host'
esql query 'FROM employees | KEEP name, salary | SORT salary DESC | LIMIT 5'
```

---

## Phase 2: Interactive REPL

A `psql`-style interactive shell for exploratory queries.

- [ ] Launch REPL when `esql` is invoked with no subcommand (or `esql repl`)
- [ ] Prompt: `esql> `
- [ ] Line editing (arrow keys, home/end, ctrl-a/e)
- [ ] Command history (persisted to `~/.esql_history`)
- [ ] Multiline query input (detect incomplete pipes, allow `\` continuation)
- [ ] Special REPL commands:
  - `\q` or `exit` — quit
  - `\f table|json|csv` — switch output format
  - `\timing` — toggle query duration display
  - `\clear` — clear screen
- [ ] Display query execution time after each result
- [ ] Graceful Ctrl-C (cancel current query, don't exit REPL)

---

## Phase 3: Connection Management [DONE]

Make it easy to work with multiple clusters.

- [x] `esql init` — interactive setup wizard (prompt for host, auth method, test connection)
- [x] Named profiles in config file:
  ```yaml
  default_profile: production
  profiles:
    production:
      host: https://prod.es.example.com:9200
      api_key: "..."
    staging:
      host: https://staging.es.example.com:9200
      user: elastic
      password: "..."
  ```
- [x] `--profile` flag to select a profile: `esql query --profile staging '...'`
- [x] `esql profiles list` — show configured profiles (with `*` marking the default)
- [x] Connection test during init (fails gracefully, doesn't save broken profiles)

---

## Phase 4: Index Introspection

Help users discover what they can query.

- [ ] `esql indices` — list available indices (hit `_cat/indices` or `_resolve/index/*`)
- [ ] `esql fields <index>` — show field names and types for an index (hit `<index>/_mapping`)
- [ ] In REPL: `\d` to list indices, `\d <index>` to show fields (like `psql`'s `\dt` and `\d`)
- [ ] Tab completion for index names in the REPL (after fetching index list)

---

## Phase 5: Full ES|QL Language Coverage

Ensure every ES|QL construct works through the CLI. The CLI passes queries verbatim to Elasticsearch, so language support is mostly about the REPL experience — autocompletion, syntax hints, validation feedback.

### Source commands
- [ ] `FROM` — works natively (passthrough)
- [ ] `ROW` — works natively
- [ ] `SHOW` — works natively

### Processing commands
- [ ] `WHERE` — works natively
- [ ] `EVAL` — works natively
- [ ] `STATS ... BY` — works natively (including per-agg `WHERE`)
- [ ] `SORT` — works natively
- [ ] `LIMIT` — works natively
- [ ] `KEEP` — works natively
- [ ] `DROP` — works natively
- [ ] `RENAME` — works natively
- [ ] `DISSECT` — works natively
- [ ] `GROK` — works natively
- [ ] `ENRICH` — works natively (requires server-side enrich policy)
- [ ] `LOOKUP JOIN` — works natively
- [ ] `MV_EXPAND` — works natively
- [ ] `INLINE STATS` — works natively
- [ ] `CHANGE_POINT` — works natively
- [ ] `FORK` — works natively (preview)
- [ ] `FUSE` — works natively (preview)
- [ ] `SAMPLE` — works natively (preview)
- [ ] `RERANK` — works natively (preview)
- [ ] `COMPLETION` — works natively (preview)

### Operators
- [ ] Binary: `==`, `!=`, `<`, `<=`, `>`, `>=`, `+`, `-`, `*`, `/`, `%`
- [ ] Logical: `AND`, `OR`, `NOT`
- [ ] Null: `IS NULL`, `IS NOT NULL`
- [ ] Pattern: `LIKE`, `RLIKE`, `IN`
- [ ] Match: `:` (full-text match)
- [ ] Cast: `::` (type conversion)

### Functions (all passthrough — key ones to validate)
- [ ] Aggregation: `AVG`, `COUNT`, `COUNT_DISTINCT`, `MAX`, `MIN`, `SUM`, `MEDIAN`, `PERCENTILE`, `STD_DEV`, `VARIANCE`, `TOP`, `VALUES`, `FIRST`, `LAST`
- [ ] Grouping: `BUCKET`, `TBUCKET`, `CATEGORIZE`
- [ ] Conditional: `CASE`, `COALESCE`, `GREATEST`, `LEAST`
- [ ] Date/time: `DATE_DIFF`, `DATE_EXTRACT`, `DATE_FORMAT`, `DATE_PARSE`, `DATE_TRUNC`, `NOW`
- [ ] String: `CONCAT`, `LENGTH`, `SUBSTRING`, `LEFT`, `RIGHT`, `TRIM`, `TO_LOWER`, `TO_UPPER`, `SPLIT`, `REPLACE`, `STARTS_WITH`, `ENDS_WITH`, `CONTAINS`
- [ ] Math: `ABS`, `ROUND`, `CEIL`, `FLOOR`, `SQRT`, `POW`, `LOG`, `LOG10`
- [ ] Type conversion: `TO_INTEGER`, `TO_DOUBLE`, `TO_STRING`, `TO_BOOLEAN`, `TO_DATETIME`, `TO_IP`, `TO_LONG`, `TO_VERSION`
- [ ] Multivalue: `MV_AVG`, `MV_COUNT`, `MV_FIRST`, `MV_LAST`, `MV_MAX`, `MV_MIN`, `MV_SUM`, `MV_SORT`, `MV_CONCAT`, `MV_EXPAND`
- [ ] Search: `MATCH`, `MATCH_PHRASE`, `KQL`, `QSTR`

> **Note:** Since the CLI sends the query string verbatim to Elasticsearch, all commands/functions/operators work as long as the server supports them. The checklist above tracks **validation and testing**, not implementation. The real work is in Phases 1–4.

---

## Phase 6: Unix Integration & Scripting

Make `esql` a first-class citizen in shell pipelines and automation.

- [ ] **Stdin query input** — read query from stdin: `echo "FROM logs | LIMIT 5" | esql query -`
- [ ] **TSV output** — tab-separated (default when piped), clean for `cut`, `awk`, `sort`
- [ ] **Streaming / chunked output** — start printing rows as they arrive for large result sets
- [ ] **`--quiet`** — suppress everything except data (no banners, no timing, no warnings)
- [ ] **`--delimiter`** — custom field delimiter for flat output
- [ ] **`--columns`** — client-side column selection (like `cut` but aware of column names)
- [ ] **Exit codes** — structured: 0 = success, 1 = query error, 2 = connection error, 3 = config error
- [ ] **`--timeout`** — per-query timeout override
- [ ] **Multi-query from file** — `esql query -f queries.esql` runs each query separated by `;` or blank lines

**Goal:** after Phase 6, this all works naturally:

```bash
# Pipeline: query → filter → aggregate in bash
esql query 'FROM access_logs | KEEP method, path, status | LIMIT 10000' | grep POST | wc -l

# Feed into other tools
esql query 'FROM metrics | STATS avg_cpu = AVG(cpu) BY host' -f json | jq '.[] | select(.avg_cpu > 80)'

# Script automation
for idx in logs-2025.01 logs-2025.02 logs-2025.03; do
  count=$(esql query "FROM $idx | STATS c = COUNT(*)" -f csv --no-header)
  echo "$idx: $count documents"
done

# Query from file
esql query -f dashboard_queries.esql --format json > results.json
```

---

## Phase 7: Polish & Distribution

- [ ] Shell completion scripts (Cobra gives us these for free, just wire them up)
- [ ] `--version` flag with build info (git sha, build date)
- [ ] Homebrew formula
- [ ] Goreleaser config for cross-platform binaries (linux/mac/windows, amd64/arm64)
- [ ] CI: GitHub Actions for test, lint, build, release
- [ ] Man page generation (Cobra supports this)
- [ ] Error messages with actionable suggestions (e.g., "connection refused — is --host correct?")
- [ ] `--verbose` / `--debug` flag for request/response tracing

---

## Non-goals (explicitly out of scope)

- **Query parsing or validation client-side** — the CLI is a thin transport layer. Elasticsearch validates the ES|QL. We don't reimplement the parser.
- **Local query execution** — we don't run ES|QL against local files. This is strictly a remote client.
- **Dashboard / TUI** — we may add a richer TUI later, but the core is a CLI that plays well with pipes.
- **Ingest / write operations** — this tool is read-only. No indexing, no delete, no bulk.
