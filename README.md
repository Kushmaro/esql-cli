# esql

A command-line interface for querying Elasticsearch using [ES|QL](https://www.elastic.co/docs/reference/query-languages/esql/) — Elastic's pipe-based query language.

Think `psql` for Postgres or `mysql` for MySQL, but for ES|QL against Elasticsearch clusters.

## What it does

- Run ES|QL queries against a remote Elasticsearch cluster from your terminal
- Interactive REPL mode for exploratory querying
- One-shot mode for scripting and piping (`esql query "FROM logs | LIMIT 10"`)
- Tabular, JSON, and CSV output formats
- Connection profiles for managing multiple clusters

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
| ES transport       | `net/http` (stdlib)     | ES|QL is a single REST endpoint (`POST _query`). No need for a heavyweight SDK |
| Table rendering    | `tablewriter`           | Clean ASCII/Unicode table output in the terminal                          |
| Interactive REPL   | TBD (`readline` / `bubbletea`) | Line editing, history, multiline input for the interactive mode    |

### Why Cobra over alternatives

Cobra was chosen over `urfave/cli`, `kong`, and bare `flag` because:

1. **Subcommand structure** — the tool naturally splits into `query`, `connect`, `indices`, and a default REPL mode
2. **Viper integration** — connection config (host, auth, TLS) needs to merge from `~/.esql.yaml`, env vars (`ESQL_HOST`, `ESQL_USER`), and flags (`--host`, `--user`) seamlessly
3. **Shell completions** — generated for bash/zsh/fish/powershell for free
4. **Ecosystem familiarity** — any Go developer will recognize the project structure immediately

### Why raw `net/http` over `go-elasticsearch`

The ES|QL API surface is a single endpoint:

```
POST /_query
{
  "query": "FROM logs | WHERE status >= 400 | LIMIT 10"
}
```

Pulling in the full `elastic/go-elasticsearch` SDK for one REST call adds unnecessary dependency weight. We use `net/http` with a thin wrapper that handles auth, TLS, and response parsing.

### Project layout

```
esql/
├── main.go                 # Entrypoint
├── cmd/                    # Cobra command definitions
│   ├── root.go             # Root command (launches REPL by default)
│   ├── query.go            # One-shot query execution
│   └── init.go             # Connection initialization
├── internal/
│   ├── client/             # Elasticsearch HTTP client (ES|QL endpoint)
│   ├── config/             # Viper config loading and profiles
│   ├── formatter/          # Output formatting (table, JSON, CSV)
│   └── repl/               # Interactive REPL
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

## Configuration

Connection settings are resolved in this order (later wins):

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
go build -o esql .
```

## License

TBD
