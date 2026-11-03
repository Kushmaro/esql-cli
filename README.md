# esql

A command-line interface for querying Elasticsearch using [ES|QL](https://www.elastic.co/docs/reference/query-languages/esql/) — Elastic's pipe-based query language.

Think `psql` for Postgres or `mysql` for MySQL, but for ES|QL against Elasticsearch clusters.

## What it does

- Run ES|QL queries against a remote Elasticsearch cluster from your terminal
- Interactive REPL mode for exploratory querying
- One-shot mode for scripting and piping (`esql query "FROM logs | LIMIT 10"`)
- Table, JSON, NDJSON, CSV, TSV, and Parquet output formats
- Complete index-to-Parquet exports using point-in-time pagination
- Named cluster connections, with credentials kept in the OS keychain

## Installation

Download a prebuilt binary for macOS, Linux, or Windows from the
[releases page](https://github.com/Kushmaro/esql-cli/releases), or install with Go 1.25+:

```bash
go install github.com/Kushmaro/esql-cli@latest
```

`go install` names the binary after the module, so it is installed as `esql-cli`.
Rename it or add `alias esql=esql-cli` to use the `esql` name shown below.

Then set up a connection:

```bash
esql init
```

## Example

```bash
# One-shot query
esql query 'FROM logs-* | WHERE status >= 400 | STATS count = COUNT(*) BY host | SORT count DESC | LIMIT 10'

# Interactive REPL
esql
esql> FROM employees | WHERE department == "engineering" | KEEP name, title, hire_date | SORT hire_date DESC
```

## Architecture

### Tech stack

| Concern            | Choice                  | Rationale                                                                 |
|--------------------|-------------------------|---------------------------------------------------------------------------|
| Language           | Go                      | Single binary distribution, strong concurrency, CLI ecosystem             |
| CLI framework      | Cobra                   | Industry standard (`kubectl`, `docker`, `gh`). Subcommand tree, flags, shell completions out of the box |
| Config management  | Viper                   | Cobra companion. Merges config files, env vars, and flags into one layer  |
| ES transport       | `net/http` (stdlib)     | ES|QL queries use `POST _query`; complete exports use PIT and search APIs |
| Table rendering    | `tablewriter`           | Clean ASCII/Unicode table output in the terminal                          |
| Interactive REPL   | `liner`                 | Line editing, history, and tab completion for the interactive mode        |

### Why Cobra over alternatives

Cobra was chosen over `urfave/cli`, `kong`, and bare `flag` because:

1. **Subcommand structure** — the tool naturally splits into `query`, `clusters`, `export`, and a default REPL mode
2. **Viper integration** — connection config (host, auth, TLS) needs to merge from `~/.esql.yaml`, env vars (`ESQL_HOST`, `ESQL_USER`), and flags (`--host`, `--user`) seamlessly
3. **Shell completions** — generated for bash/zsh/fish/powershell for free
4. **Ecosystem familiarity** — any Go developer will recognize the project structure immediately

### Why raw `net/http` over `go-elasticsearch`

The core ES|QL query endpoint is:

```
POST /_query
{
  "query": "FROM logs | WHERE status >= 400 | LIMIT 10"
}
```

We use `net/http` with a thin wrapper that handles auth, TLS, and response parsing. Metadata commands use the relevant discovery APIs; complete exports use point-in-time search with `search_after`.

### Project layout

```
esql/
├── main.go                 # Entrypoint
├── cmd/                    # Cobra command definitions
│   ├── root.go             # Root command (launches REPL by default)
│   ├── query.go            # One-shot query execution
│   ├── init.go             # Interactive connection setup
│   └── ...                 # clusters, queries, export, async, views, promql, ...
├── internal/
│   ├── client/             # Elasticsearch HTTP client
│   ├── config/             # Viper config loading, clusters, keyring credentials
│   ├── exporter/           # Point-in-time index exports
│   ├── formatter/          # Output formatting (table, JSON, CSV, TSV, Parquet)
│   └── repl/               # Interactive REPL
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

## Native parameters and query settings

Bind values without modifying query text. `--bind` values are JSON, so quote
string values inside the shell argument:

```bash
./esql query 'ROW n = ?n, name = ?name' --bind n=42 --bind 'name="Ada"'
./esql query 'ROW n = ?, enabled = ?' --params '[42,true]'
./esql query 'ROW n = ?n' --params @params.json --settings @settings.json
./esql query 'ROW n = 1' --setting 'time_zone="Europe/Amsterdam"' --setting approximation=false
./esql query 'ROW n = 1' --settings '{"column_metadata":true}' --profile --raw
```

`--params` accepts the native Elasticsearch parameter array, including named
and typed bindings. It cannot be combined with `--bind`. `--settings` accepts
a JSON object; repeated `--setting name=JSON` entries can add distinct settings.
Duplicate names and malformed JSON fail before query execution. All JSON inputs
preserve integer precision. `--time-zone` and `--locale` also expose the top-level
request options; `settings.time_zone` takes precedence over `--time-zone`.

These flags also work with `queries run`, `watch`, and `async submit`. The existing
`--param name=value` option continues to substitute `{{name}}` text and can be
combined with native bindings. Use native bindings for data values.

Normal JSON output remains an array of result rows. `--raw` returns the complete
Elasticsearch response, including profiling, execution counters and column
metadata. Add `--execution-metadata` to request cluster execution details.

## Async queries and monitoring

```bash
# Returns a full JSON response with id and is_running
./esql async submit 'ROW n = ?n' --bind n=42 --wait-for-completion 0s

# Check status immediately, or wait up to 5 seconds for results
./esql async get '<async-id>'
./esql async get '<async-id>' --wait-for-completion 5s

# Format completed result rows using any existing output format
./esql async get '<async-id>' --format parquet -o results.parquet

# Stop execution and retrieve available results; then release stored results
./esql async stop '<async-id>'
./esql async delete '<async-id>'

# Inspect running queries (requires monitor_esql cluster privileges)
./esql running
./esql running '<monitoring-id>'
```

Async submit/get/stop return the **full response JSON by default**, preserving
IDs, running/partial status and metadata. An explicit `--format` selects result
rows instead. Pending queries cannot be formatted as completed rows; Parquet
also rejects partial results. A failed or pending Parquet retrieval leaves an
existing output file intact.

Submissions default to `--keep-on-completion=true` and `--keep-alive 5d`, so even
fast queries retain an ID. Use `async delete` to release results when finished,
or set `--keep-on-completion=false` to retain only queries that outlast the
initial wait. `async get --keep-alive 1h` extends retention. `--timeout` controls
the HTTP request timeout and should exceed `--wait-for-completion` for a long wait.
Async queries do not provide an unlimited export cursor; use `export` for that.

Pass a query ID from the keys in `running` output to inspect active execution.
Completed async queries leave the running list; use `async get` for retained results. `running` and `async delete` also accept `--output` to save their
JSON responses.

## Stored views

Manage shared Elasticsearch view definitions with `views` (`view` is an alias):

```bash
./esql views list
./esql views put my-view 'ROW n = 42'
./esql views get my-view
./esql query 'FROM my-view'
./esql views put my-view --file definition.esql
./esql views delete my-view
```

`put` creates or replaces a view. It accepts a query argument, `-` for stdin, or
`--file`, preserving the complete query including semicolons and quoted strings.
`get` and `delete` accept several names or a comma-separated list. API responses
are full JSON; `--output` saves the response and `--timeout` controls the HTTP wait.
These are server views; `save` and `queries` continue to manage local YAML queries.

The server requires `read_view_metadata` for list/get, `create_view` for put, and
`delete_view` for deletion. Executing a view also requires the corresponding
query privileges. Optional `--description` sends the documented description
field only when requested. **The tested 9.5.5 deployment rejects that field**;
omit it there. The CLI surfaces the server error instead of dropping the value.

## PromQL HTTP API

```bash
./esql promql query 'vector(1)'
./esql promql query 'up{job="api"}' --index 'metrics-*' --time 2026-10-18T00:00:00Z
./esql promql range 'vector(2)' --start 1792281600 --end 1792281660 --step 30s
./esql promql labels --match 'up{job="api"}'
./esql promql label-values job --match up
./esql promql series --match up --match 'http_requests_total{job="api"}'
./esql promql metadata --metric up --limit 10 --limit-per-metric 1
./esql promql buildinfo
```

All seven commands use `/_prometheus/` and return the **full Prometheus JSON
response**, preserving warnings, labels, sample strings, and special values
such as `NaN`. They do not convert responses into ES|QL result rows.
`--output` writes JSON to a file. `--index` supports an index expression or alias;
without it the server uses its default `metrics-*` scope. Stored metrics must
use time-series mappings supported by Elasticsearch's PromQL implementation.

Instant queries accept `--time`. Range queries require `--start`, `--end`, and
`--step`. Timestamps can be RFC3339 strings or Unix seconds, including fractions;
steps can be durations or seconds. `--query-timeout` is the server evaluation
budget, while `--timeout` is the HTTP request timeout. `--limit` bounds query or
discovery results on routes that support it; zero uses the server's own cap.

Discovery commands accept repeated `--match` selectors (sent as `match[]`),
optional `--start`/`--end`, and `--limit`. `series` requires at least one selector.
Selectors containing commas remain intact. Empty selectors and negative limits
are rejected before making requests.

GET is the default. Query, range, labels, and series also support
`--method POST` with form-encoded parameters. POST requires the deployment to
accept authenticated forms over Elasticsearch HTTP TLS; use GET if the server
rejects form requests. Label-values, metadata, and buildinfo use GET only.
Both GET and supported POST forms passed tests on the supplied deployment.

You can still send a `PROMQL ...` source command through `esql query` to use
ES|QL post-processing. The REPL now completes `PROMQL` and `TS` source commands.

## Parquet exports

Export **every document** in an index, alias, data stream, pattern, or comma-separated list:

```bash
esql export 'logs-*' --output logs.parquet
esql export 'index-a,index-b' -o data.parquet --page-size 500 --timeout 2m
```

This uses a point-in-time snapshot and paginates beyond 10,000 documents. It
requires index `read` privileges and enabled `_source`. Rows contain `_index`,
`_id`, and top-level source fields. Arrays become native lists; objects and
fields with conflicting types are JSON strings. Source date strings remain
strings. Missing and explicit null fields both become Parquet nulls.

Schema discovery spools documents to a temporary file before writing Parquet;
allow enough temporary disk space for the source data plus the output file.
Memory is bounded by a search page and Parquet row group. `--timeout` applies to
each HTTP request. Ctrl-C cancels the export and cleans up the PIT and temporary
files. The destination file is replaced only after a successful export.
This exports document data; index mappings and settings are not included.

Export a filtered or aggregated **ES|QL result** using the existing query command:

```bash
esql query 'FROM logs-* | KEEP @timestamp, message | LIMIT 10000' -f parquet -o results.parquet
esql queries run my-query -f parquet -o results.parquet
```

Query-result exports preserve numeric types, signed and unsigned 64-bit integers,
nulls, multivalue fields, and timestamp precision. They obey Elasticsearch's
ES|QL result limits; use `export` for complete indices. One query is allowed per
Parquet file. Binary output must go to `--output` or redirected stdout.

See [Elasticsearch 9.5.5 API coverage](API-COVERAGE.md) for the audited API gaps
and deployment verification results.

## Configuration

The easiest way to add a cluster is `esql init`, which prompts for the host and
credentials, tests the connection, and saves a named cluster to `~/.esql.yaml`.
Manage saved clusters with `esql clusters list|set|remove`, and pick one per
command with `--cluster <name>`.

### Credentials and the OS keyring

`esql init` stores passwords and API keys in the OS keyring (macOS Keychain,
Secret Service on Linux, Windows Credential Manager) under the service name
`esql-cli`. The config file then only records the non-secret settings:

```yaml
default_cluster: prod
clusters:
  prod:
    host: https://my-cluster.es.example.com:9200
    user: elastic
    keyring: true
```

If no keyring is available (for example a headless Linux box without a Secret
Service daemon), `esql init` warns and falls back to storing the credentials in
plain text in the config file, which is written with `0600` permissions.
Existing clusters with plain-text credentials keep working; re-run `esql init`
with the same cluster name to move them into the keyring.

### Ad-hoc connections

Without a named cluster, connection settings are resolved in this order (later wins):

1. Config file (`~/.esql.yaml`)
2. Environment variables (`ESQL_HOST`, `ESQL_USER`, `ESQL_PASSWORD`, `ESQL_API_KEY`)
3. CLI flags (`--host`, `--user`, `--password`, `--api-key`)

Two auth methods are supported. API key takes precedence when both are set.

```yaml
# Basic auth
host: https://my-cluster.es.example.com:9200
user: elastic
password: changeme
insecure: false

# --- OR ---

# API key auth
host: https://my-cluster.es.example.com:9200
api_key: "base64-encoded-api-key"
insecure: false
```

## Building

```bash
make build   # builds ./esql with version info
make test
```

## License

[MIT](LICENSE)
