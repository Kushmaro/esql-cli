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

## Phase 2: Connection Management [DONE]

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

## Phase 3: Index Introspection [DONE]

Help users discover what they can query.

- [x] `esql indices` — list available indices (via `_cat/indices`, supports wildcard pattern filter)
- [x] `esql datastreams` (alias: `esql ds`) — list data streams with health, index mode, backing indices, and template (via `_data_stream`)
- [x] `esql fields <index|datastream>` — show field names and types (via `_mapping`, works with both indices and data stream names)
- [x] Output format support (`--format table|json|csv|tsv`, `--no-header`) on all commands

---

## Phase 4: Unix Integration & Scripting [DONE]

Make `esql` a first-class citizen in shell pipelines and automation.

- [x] **Stdin query input** — read query from stdin: `echo "FROM logs | LIMIT 5" | esql query -`
- [x] **TSV output** — tab-separated (default when piped), clean for `cut`, `awk`, `sort`
- [x] **`--quiet`** — suppress everything except data (no banners, no timing, no warnings)
- [x] **`--delimiter`** — custom field delimiter for flat output
- [x] **`--columns`** — client-side column selection (like `cut` but aware of column names)
- [x] **Exit codes** — structured: 0 = success, 1 = query error, 2 = connection error, 3 = config error
- [x] **`--timeout`** — per-query timeout override
- [x] **Multi-query from file** — `esql query --file queries.esql` runs each query separated by `;`
- [ ] **Streaming / chunked output** — start printing rows as they arrive for large result sets (deferred — requires ES async query API)

**Goal:** after Phase 4, this all works naturally:

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

## Phase 5: Polish & Distribution

- [x] Shell completion scripts (`esql completion bash|zsh|fish|powershell`) with dynamic completions for `--profile`, `--format`, and index names
- [ ] `--version` flag with build info (git sha, build date)
- [ ] Homebrew formula
- [ ] Goreleaser config for cross-platform binaries (linux/mac/windows, amd64/arm64)
- [ ] CI: GitHub Actions for test, lint, build, release
- [ ] Man page generation (Cobra supports this)
- [ ] Error messages with actionable suggestions (e.g., "connection refused — is --host correct?")
- [ ] `--verbose` / `--debug` flag for request/response tracing

---

## Phase 6: Interactive REPL [DONE]

A `psql`-style interactive shell for exploratory queries. Launched by running `esql` with no subcommand on an interactive terminal.

### Core REPL

- [x] Launch REPL when `esql` is invoked with no subcommand (interactive TTY only; non-TTY shows help)
- [x] Prompt: `esql> ` (continuation: `  ...> `)
- [x] Line editing (arrow keys, home/end, ctrl-a/e) via `peterh/liner`
- [x] Command history (persisted to `~/.esql_history`)
- [x] Multiline query input (lines ending with `|` or `\` continue on the next line)
- [x] Display query execution time after each result (ES `took` + client round-trip)
- [x] Graceful Ctrl-C (cancel current input, don't exit REPL)
- [x] Ctrl-D to exit cleanly
- [x] Cluster info banner on connect (node name, version, cluster name)

### REPL commands

- [x] `\q` or `exit` — quit
- [x] `\f table|json|csv|tsv` — switch output format (shows current format when called with no arg)
- [x] `\timing` — toggle query duration display
- [x] `\clear` — clear screen
- [x] `\d` — list indices (excludes system dot-prefixed indices)
- [x] `\d <index>` — show fields for an index (also caches fields for completion)
- [x] `\h` — show help with all commands and keyboard shortcuts
- [x] `\refresh` — refresh index list and clear field cache

### REPL autocompletion & syntax awareness

Context-aware tab completion for the full ES|QL language. The CLI sends queries verbatim to Elasticsearch (no client-side parsing), but the REPL knows the language vocabulary for a good interactive experience.

- [x] Tab completion for index names (fetched from cluster on connect, excludes dot-prefixed system indices)
- [x] Tab completion for field names (lazy-fetched per index on first tab, cached; also pre-fetched after query execution)
- [x] Tab completion for ES|QL commands, keywords, operators, and functions (lists below)
- [x] Context-aware suggestions: source commands at start, processing commands after `|`, index names after `FROM`, aggregation functions in `STATS`, field names + functions in `WHERE`/`EVAL`, sort modifiers after `SORT`, etc.
- [x] Metacommand completion (`\q`, `\f`, `\timing`, `\clear`, `\d`, `\h`, `\refresh`)

#### ES|QL keyword reference (for autocompletion)

**Source commands:** `FROM`, `ROW`, `SHOW`, `TS`

**Processing commands:** `WHERE`, `EVAL`, `STATS`, `SORT`, `LIMIT`, `KEEP`, `DROP`, `RENAME`, `DISSECT`, `GROK`, `ENRICH`, `LOOKUP JOIN`, `MV_EXPAND`, `INLINE STATS`, `CHANGE_POINT`, `FORK`, `FUSE`, `SAMPLE`, `RERANK`, `COMPLETION`

**Clauses & keywords:** `BY`, `AS`, `ON`, `WITH`, `METADATA`, `ASC`, `DESC`, `NULLS FIRST`, `NULLS LAST`, `AND`, `OR`, `NOT`, `IN`, `LIKE`, `RLIKE`, `IS NULL`, `IS NOT NULL`, `TRUE`, `FALSE`, `NULL`, `INFO`

**Operators:** `==`, `!=`, `<`, `<=`, `>`, `>=`, `+`, `-`, `*`, `/`, `%`, `:` (match), `::` (cast)

**Aggregation functions:** `ABSENT`, `AVG`, `COUNT`, `COUNT_DISTINCT`, `FIRST`, `LAST`, `MAX`, `MEDIAN`, `MEDIAN_ABSOLUTE_DEVIATION`, `MIN`, `PERCENTILE`, `PRESENT`, `SAMPLE`, `ST_CENTROID_AGG`, `ST_EXTENT_AGG`, `STD_DEV`, `SUM`, `TOP`, `VALUES`, `VARIANCE`, `WEIGHTED_AVG`

**Time series aggregation functions:** `ABSENT_OVER_TIME`, `AVG_OVER_TIME`, `COUNT_OVER_TIME`, `COUNT_DISTINCT_OVER_TIME`, `DELTA`, `DERIV`, `FIRST_OVER_TIME`, `IDELTA`, `INCREASE`, `IRATE`, `LAST_OVER_TIME`, `MAX_OVER_TIME`, `MIN_OVER_TIME`, `PERCENTILE_OVER_TIME`, `PRESENT_OVER_TIME`, `RATE`, `STDDEV_OVER_TIME`, `SUM_OVER_TIME`, `VARIANCE_OVER_TIME`

**Grouping functions:** `BUCKET`, `TBUCKET`, `CATEGORIZE`

**Conditional functions:** `CASE`, `COALESCE`, `GREATEST`, `LEAST`, `CLAMP`, `CLAMP_MIN`, `CLAMP_MAX`

**Date/time functions:** `DATE_DIFF`, `DATE_EXTRACT`, `DATE_FORMAT`, `DATE_PARSE`, `DATE_TRUNC`, `DAY_NAME`, `MONTH_NAME`, `NOW`, `TRANGE`

**String functions:** `BIT_LENGTH`, `BYTE_LENGTH`, `CHUNK`, `CONCAT`, `CONTAINS`, `ENDS_WITH`, `FROM_BASE64`, `HASH`, `LEFT`, `LENGTH`, `LOCATE`, `LTRIM`, `MD5`, `REPEAT`, `REPLACE`, `REVERSE`, `RIGHT`, `RTRIM`, `SHA1`, `SHA256`, `SPACE`, `SPLIT`, `STARTS_WITH`, `SUBSTRING`, `TO_BASE64`, `TO_LOWER`, `TO_UPPER`, `TRIM`, `URL_DECODE`, `URL_ENCODE`, `URL_ENCODE_COMPONENT`

**Math functions:** `ABS`, `ACOS`, `ACOSH`, `ASIN`, `ASINH`, `ATAN`, `ATAN2`, `ATANH`, `CBRT`, `CEIL`, `COPY_SIGN`, `COS`, `COSH`, `E`, `EXP`, `FLOOR`, `HYPOT`, `LOG`, `LOG10`, `PI`, `POW`, `ROUND`, `ROUND_TO`, `SCALB`, `SIGNUM`, `SIN`, `SINH`, `SQRT`, `TAN`, `TANH`, `TAU`

**IP functions:** `CIDR_MATCH`, `IP_PREFIX`

**Spatial functions:** `ST_CONTAINS`, `ST_DISJOINT`, `ST_DISTANCE`, `ST_ENVELOPE`, `ST_GEOHASH`, `ST_GEOHEX`, `ST_GEOTILE`, `ST_INTERSECTS`, `ST_NPOINTS`, `ST_SIMPLIFY`, `ST_WITHIN`, `ST_X`, `ST_XMAX`, `ST_XMIN`, `ST_Y`, `ST_YMAX`, `ST_YMIN`

**Search functions:** `DECAY`, `KQL`, `MATCH`, `MATCH_PHRASE`, `QSTR`, `SCORE`, `TOP_SNIPPETS`

**Dense vector functions:** `KNN`, `TEXT_EMBEDDING`, `V_COSINE`, `V_DOT_PRODUCT`, `V_HAMMING`, `V_L1_NORM`, `V_L2_NORM`

**Type conversion functions:** `TO_BOOLEAN`, `TO_CARTESIANPOINT`, `TO_CARTESIANSHAPE`, `TO_DATEPERIOD`, `TO_DATETIME`, `TO_DATE_NANOS`, `TO_DEGREES`, `TO_DENSE_VECTOR`, `TO_DOUBLE`, `TO_GEOPOINT`, `TO_GEOSHAPE`, `TO_INTEGER`, `TO_IP`, `TO_LONG`, `TO_RADIANS`, `TO_STRING`, `TO_TIMEDURATION`, `TO_UNSIGNED_LONG`, `TO_VERSION`

**Multivalue functions:** `MV_APPEND`, `MV_AVG`, `MV_CONCAT`, `MV_CONTAINS`, `MV_COUNT`, `MV_DEDUPE`, `MV_FIRST`, `MV_INTERSECTION`, `MV_INTERSECTS`, `MV_LAST`, `MV_MAX`, `MV_MEDIAN`, `MV_MEDIAN_ABSOLUTE_DEVIATION`, `MV_MIN`, `MV_PERCENTILE`, `MV_PSERIES_WEIGHTED_SUM`, `MV_SLICE`, `MV_SORT`, `MV_SUM`, `MV_UNION`, `MV_ZIP`

---

## Phase 7: Query Ergonomics

Make writing and re-running queries frictionless.

- [x] **Saved queries / bookmarks** — `esql save "error-spike" 'FROM logs-* | ...'` persists named queries to `~/.esql.yaml` (or a separate `~/.esql_queries.yaml`). `esql run error-spike` executes them. `esql queries list` shows saved queries.
- [x] **Query parameterization** — support `{{param}}` placeholders in saved queries: `esql run error-spike --param index=logs-* --param min_status=500`. Also works inline: `esql query 'FROM {{index}} | WHERE status >= {{min}}' --param index=logs-* --param min=500`
- [ ] **Watch mode** — `esql watch 'FROM metrics | STATS avg(cpu) BY host' --every 5s` re-runs a query on an interval and refreshes the table in place. Useful for live monitoring in a terminal.
- [ ] **Dry-run / generate curl** — `esql query --dry-run '...'` prints the equivalent `curl` command instead of executing. Useful for sharing with colleagues or debugging outside the CLI.

---

## Phase 8: REPL Enhancements

Bring the interactive experience closer to `psql`.

- [ ] **Syntax highlighting** — color ES|QL keywords, functions, strings, and operators as the user types in the REPL
- [ ] **Function signature hints** — when tab-completing a function like `AVG(`, show a brief inline hint with expected arguments (e.g., `AVG(number_field)`)
- [ ] **Pager support** — automatically pipe large result sets through `less` (like `psql` does), with `\pager` to toggle
- [ ] **`.esqlrc` startup file** — like `.psqlrc`, auto-run commands on REPL start (e.g., `\f json`, `\timing`)
- [ ] **`\sample <index>`** — show a quick sample of rows from an index (`FROM <index> | LIMIT 5`) in one keystroke, so you can eyeball the shape of the data without typing a full query
- [ ] **`\history`** — browse and re-run queries from history interactively
- [ ] **Horizontal scroll / column truncation** — handle wide result sets that exceed terminal width (truncate with `...` or allow horizontal scrolling)

---

## Phase 9: Output & Integration

Expand output formats and make `esql` a bridge to other tools.

- [ ] **NDJSON output format** — one JSON object per line (`--format ndjson`). The lingua franca for streaming data into jq, Logstash, custom scripts, and other systems.
- [ ] **`--output` / `-o` flag** — write results directly to a file while still showing progress/timing on stderr
- [ ] **Export to Parquet** — `--format parquet` for users who want to pull ES data into pandas, DuckDB, or Spark for offline analysis
- [ ] **Streaming / chunked output** — start printing rows as they arrive for large result sets (requires ES async query API; carried forward from Phase 4)

---

## Phase 10: Operational & CI-Friendly

Use `esql` in automation, monitoring, and CI/CD pipelines.

- [ ] **Threshold assertions** — `esql check 'FROM metrics | STATS avg_cpu = AVG(cpu)' --assert 'avg_cpu < 80'`. Exit 0 if the assertion passes, exit 1 if it fails. Use in CI/CD, cron jobs, or alerting scripts.
- [ ] **Query performance logging** — `--timing-log <file>` appends `{query, took_ms, timestamp}` to a file after each execution for tracking query performance over time
- [ ] **Health check / ping** — `esql ping` tests connectivity and exits with 0/1. Useful for Docker health checks, readiness probes, and scripts.

---

## Phase 11: Discovery & Exploration

Help users understand their clusters and data.

- [ ] **Cluster status** — `esql status` showing cluster health, node count, total indices/docs/size at a glance
- [ ] **Schema comparison** — `esql diff staging production` compares field mappings between two profiles/clusters. Catches mapping drift between environments.

---

## Non-goals (explicitly out of scope)

- **Query parsing or validation client-side** — the CLI is a thin transport layer. Elasticsearch validates the ES|QL. We don't reimplement the parser.
- **Local query execution** — we don't run ES|QL against local files. This is strictly a remote client.
- **Dashboard / TUI** — we may add a richer TUI later, but the core is a CLI that plays well with pipes.
- **Ingest / write operations** — this tool is read-only. No indexing, no delete, no bulk.
