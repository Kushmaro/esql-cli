# Elasticsearch 9.5.5 API coverage

Audited on 2026-10-18 against this repository and the supplied deployment. `GET /` confirmed **9.5.5**. Credentials and the deployment URL are excluded from this report.

This is an ES|QL client, so the comparison covers ES|QL APIs and the adjacent PromQL query APIs. It does not enumerate unrelated Elasticsearch administration APIs. Query language syntax, REST request options, and dedicated CLI commands are separate kinds of support.

## Implemented in the first API expansion

Native named/positional/typed parameters (`--bind`, `--params`), query settings
(`--setting`, `--settings`), top-level time zone/locale, profiling, execution
metadata, and full-response output (`--raw`) are now supported. They work with
one-shot queries, saved queries, watch mode, and async submission. The existing
`--param` text-substitution option remains available.

All four async lifecycle operations are implemented as `async submit`, `get`,
`stop`, and `delete`. Running-query list/detail APIs are implemented as `running
[id]`. Async responses default to full JSON; explicit formats produce completed
result rows. Pending or partial Parquet results are rejected.

Live validation on 9.5.5 passed named/positional parameters, exact INT64 values,
an injection-like string passed unchanged, Amsterdam time-zone execution,
profiling, structured settings, and async submit/get/stop/delete. The async
submission returned `is_running: true`, its completed results exported to Parquet,
and a get after deletion returned 404. Running-query listing returned 200.
Successful individual-query inspection is covered with a local HTTP server.
A live request using a valid async ID after the query had completed returned
HTTP 500 with `task ... isn't running and hasn't stored its results`; this
server error is surfaced unchanged by the CLI. No long-running live query was
created for a successful active-detail check. All test async results were
deleted. Saved-query and watch execution also passed live constant-query checks,
including combining legacy substitution with native bindings/settings.
No existing index documents were read for these checks.

The remaining endpoint gaps after that first batch were stored views and the
unavailable external source/dataset routes. Views and PromQL are covered below.

## Stored-view and PromQL expansion

Stored views now have `views list`, `get`, `put`, and `delete` commands, covering
all documented GET/PUT/DELETE routes. Inputs preserve complete query text from
arguments, stdin or files. Multi-name get/delete, JSON output, connection profiles
and request timeouts are supported. Descriptions are sent only when explicitly
requested. On this **9.5.5 deployment**, PUT rejected the documented optional
`description` field with HTTP 400 (`unknown field [description]`); query-only
view definitions work. No description is silently discarded.

Live view checks passed: create two uniquely named views, list, multi-name get,
execute through `FROM`, replace a definition from a file, preserve an exact
INT64 and semicolon-containing text, then multi-name delete. Both test names
were confirmed absent afterward.

The PromQL HTTP API now has `promql query`, `range`, `labels`, `label-values`,
`series`, `metadata`, and `buildinfo`. All support index scoping and full JSON
output. Query/range/labels/series support GET and correctly form-encoded POST;
other routes enforce GET. Sample strings, warnings and response fields are
preserved. Native Prometheus error envelopes get readable messages and nonzero
exit codes, with PromQL-specific guidance. REPL completion includes `PROMQL`/`TS`.

Live checks passed all seven GET families, index-scoped queries, all four
supported POST routes, expected constant instant/range values, repeated discovery
selectors, and invalid-query failure behavior. A uniquely named temporary
`time_series` index with three synthetic gauge samples also passed: instant
query returned the latest value, GET and POST range queries returned exactly
`1, 2, 3`, label-name/value and series discovery matched the fixture, and metric
metadata reported `gauge`. The index was deleted and its absence verified.
No existing views or index templates were changed. All temporary views and
indices created during validation were removed.

## Original audit: missing ES|QL endpoints

Before the expansion above, the CLI implemented synchronous `POST /_query` only. All 15 other operations in the [ES|QL API catalog](https://www.elastic.co/docs/api/doc/elasticsearch/group/endpoint-esql) lacked dedicated client/CLI support:

| API family | Missing operations | Deployment check |
| --- | --- | --- |
| Async execution | `POST /_query/async`, `GET /_query/async/{id}`, `DELETE /_query/async/{id}`, `POST /_query/async/{id}/stop` | All four returned 200 using a constant-only `ROW` query; stored result deleted afterward. |
| Running-query monitoring | `GET /_query/queries`, `GET /_query/queries/{id}` | List returned 200. Individual query inspection was not exercised because no long-running query was created. |
| Stored views | `GET /_query/view[/{name}]`, `PUT /_query/view/{name}`, `DELETE /_query/view/{name}` | List returned 200. Mutations were not performed. Local `esql save` stores query text in YAML and does not manage server views. |
| External data sources | `GET /_query/data_source[/{name}]`, `PUT /_query/data_source/{name}`, `DELETE /_query/data_source/{name}` | List returned 400, **no handler found**. Documented, but unavailable on this deployment. |
| External datasets | `GET /_query/dataset[/{name}]`, `PUT /_query/dataset/{name}`, `DELETE /_query/dataset/{name}` | List returned 400, **no handler found**. Documented, but unavailable on this deployment. |

Monitoring is documented as experimental since 9.1, views since 9.4, and the data-source/dataset APIs since 9.5. Documentation availability does not prove that an experimental route is enabled in a particular build. The reason for the two missing handlers was not established. See [monitoring](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-esql-get-query), [views](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-esql-get-view), [data sources](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-esql-get-data-source), and [datasets](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-esql-get-dataset).

Async request options such as `keep_on_completion` and `wait_for_completion_timeout` belong in the JSON body on this deployment; sending them as URL parameters returned 400. Async execution does not provide an unlimited document-export cursor.

## Current synchronous query controls still missing

The structured request now carries native parameters/settings and the execution
options described above. `--raw` retains the complete response, including unknown
version-specific fields and column `_meta`. Remaining controls from the
[query API](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-esql-query):

| Missing control | Current limitation |
| --- | --- |
| Query DSL `filter`, inline `tables` | No structured request fields or flags yet. |
| `include_ccs_metadata` | No separate CCS-specific flag; `--execution-metadata` requests the broader execution metadata. |
| `columnar`, `drop_null_columns` | No response-layout controls. |
| `allow_partial_results` | No ES|QL request flag; Parquet exports reject a returned `is_partial: true`. |
| Native response formats | JSON transport with local formatting; no Arrow, YAML, CBOR, or Smile passthrough. |

## Query-language UX gaps

A directly supplied `esql query '...'` is sent verbatim, so a missing completion entry does **not** prevent execution of a server-supported command or function.

- `TS`, `PROMQL`, and `SET` are accepted when saving queries. Source completion now includes `TS` and `PROMQL`.
- Current processing commands missing from completion include `DEDUP`, `DENSE_VECTOR`, `HIGHLIGHT`, `METRICS_INFO`, `IP_LOCATION`, `MMR`, `REGISTERED_DOMAIN`, `TS_INFO`, `TS_COLLAPSE`, `USER_AGENT`, and `URI_PARTS`. These are documentation comparisons, not individual live execution checks.
- File/stdin query splitting uses every semicolon as a query boundary. That breaks `SET ...; FROM ...` directives and semicolons inside string literals. A single direct argument bypasses this splitter; a constant `SET time_zone ...; ROW ...` probe returned 200.

Sources: [source commands](https://www.elastic.co/docs/reference/query-languages/esql/commands/source-commands), [processing commands](https://www.elastic.co/docs/reference/query-languages/esql/commands/processing-commands), [SET syntax](https://www.elastic.co/docs/reference/query-languages/esql/directives/set). Code: `internal/repl/keywords.go`, `cmd/queries.go:looksLikeQuery`, and `cmd/query.go:splitQueries`.

## Adjacent PromQL HTTP APIs

All seven documented endpoint families now have CLI commands, including
index-scoped paths and form POST on supported routes. See the
[PromQL HTTP API](https://www.elastic.co/docs/reference/query-languages/promql/promql-http-api)
and the README examples. Full JSON output retains the native response shape.

## Parquet work completed

The existing feature originated in commit `ea9b187` (`Add NDJSON, Parquet output formats and --output/-o file flag`). It was a query-result formatter, not a complete index exporter.

- Fixed exact signed/unsigned integer handling, UTF-8 string annotations, native lists, timestamps, nulls, zero/false values, column order, empty results, and useful conversion/write errors.
- Prevented concatenating multiple queries into one invalid Parquet file and prevented writing binary output to a terminal.
- Query and saved-query file exports now publish atomically, preserving an existing destination on failure. Parquet rejects partial ES|QL responses.
- Added `esql export <index-expression> -o output.parquet` using PIT plus `search_after`, with no ES|QL result limit. A temporary spool discovers the schema across every page while keeping memory bounded. PITs and temporary files are cleaned up on errors and cancellation.
- Full exports include `_index`, `_id`, and top-level source fields. Source arrays become lists; objects/nested structures and conflicting field types become JSON strings. Source dates remain strings because this path infers source types without mappings; query-result dates use native timestamps. Missing and null fields both become Parquet nulls. Requires `_source`; this is a data export, not an index-settings/mappings backup.

The [ES|QL result cap](https://www.elastic.co/docs/reference/query-languages/esql/limitations) still applies to `query --format parquet` (normally 1,000 rows by default and at most 10,000 for regular queries, configurable on the server). Complete exports instead use the [recommended PIT/search-after pagination](https://www.elastic.co/docs/reference/elasticsearch/rest-apis/paginate-search-results).

## Validation

- `go test -race ./...`: passed, including 10,003-document pagination, rotated PIT IDs, cleanup/cancellation, failure atomicity, schema discovery, and Parquet file read-back.
- `go vet ./...`: passed.
- Live synthetic ES|QL Parquet export: succeeded; file read-back checks exact INT64, UINT64, lists, and timestamps.
- Live complete export: **11,888 documents** from the two explicitly approved Fleet metrics indices, using 500-document pages. Counts before and after were both 11,888. The resulting file was read back successfully: 11,888 distinct `(index, id)` identities, eight columns, one row group, and 5,944 documents per index. Output: `/private/tmp/esql-live-indices.parquet` (6,762,682 bytes).

## Remaining follow-up order

1. Query DSL filters, inline tables, partial-result controls, and response-layout/native-format options.
2. Source/directive completion and a quote/directive-aware query splitter.
3. Data-source/dataset management after confirming deployment availability.
