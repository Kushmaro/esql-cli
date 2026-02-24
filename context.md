# ES|QL API Reference

This CLI sends ES|QL queries verbatim to Elasticsearch. The following links document the full ES|QL query language. Append `.md` to any URL to retrieve it as markdown.

## Core reference

- **Syntax**: https://www.elastic.co/docs/reference/query-languages/esql/esql-syntax
- **Commands overview**: https://www.elastic.co/docs/reference/query-languages/esql/esql-commands
- **Functions & operators**: https://www.elastic.co/docs/reference/query-languages/esql/esql-functions-operators

## Source commands

- **FROM**: https://www.elastic.co/docs/reference/query-languages/esql/commands/from
- **ROW**: https://www.elastic.co/docs/reference/query-languages/esql/commands/row
- **SHOW**: https://www.elastic.co/docs/reference/query-languages/esql/commands/show
- **TS**: https://www.elastic.co/docs/reference/query-languages/esql/commands/ts

## Processing commands

- **Source commands list**: https://www.elastic.co/docs/reference/query-languages/esql/commands/source-commands
- **Processing commands list**: https://www.elastic.co/docs/reference/query-languages/esql/commands/processing-commands
- **WHERE**: https://www.elastic.co/docs/reference/query-languages/esql/commands/where
- **EVAL**: https://www.elastic.co/docs/reference/query-languages/esql/commands/eval
- **STATS**: https://www.elastic.co/docs/reference/query-languages/esql/commands/stats-by
- **INLINE STATS**: https://www.elastic.co/docs/reference/query-languages/esql/commands/inlinestats-by
- **SORT**: https://www.elastic.co/docs/reference/query-languages/esql/commands/sort
- **LIMIT**: https://www.elastic.co/docs/reference/query-languages/esql/commands/limit
- **KEEP**: https://www.elastic.co/docs/reference/query-languages/esql/commands/keep
- **DROP**: https://www.elastic.co/docs/reference/query-languages/esql/commands/drop
- **RENAME**: https://www.elastic.co/docs/reference/query-languages/esql/commands/rename
- **DISSECT**: https://www.elastic.co/docs/reference/query-languages/esql/commands/dissect
- **GROK**: https://www.elastic.co/docs/reference/query-languages/esql/commands/grok
- **ENRICH**: https://www.elastic.co/docs/reference/query-languages/esql/commands/enrich
- **LOOKUP JOIN**: https://www.elastic.co/docs/reference/query-languages/esql/commands/lookup-join
- **MV_EXPAND**: https://www.elastic.co/docs/reference/query-languages/esql/commands/mv_expand
- **CHANGE_POINT**: https://www.elastic.co/docs/reference/query-languages/esql/commands/change-point
- **FORK**: https://www.elastic.co/docs/reference/query-languages/esql/commands/fork
- **FUSE**: https://www.elastic.co/docs/reference/query-languages/esql/commands/fuse
- **SAMPLE**: https://www.elastic.co/docs/reference/query-languages/esql/commands/sample
- **RERANK**: https://www.elastic.co/docs/reference/query-languages/esql/commands/rerank
- **COMPLETION**: https://www.elastic.co/docs/reference/query-languages/esql/commands/completion

## Functions & operators (by category)

- **Operators**: https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/operators
- **Aggregation functions**: https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/aggregation-functions
- **Grouping functions**: https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/grouping-functions
- **Conditional functions**: https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/conditional-functions-and-expressions
- **Date/time functions**: https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/date-time-functions
- **String functions**: https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/string-functions
- **Math functions**: https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/math-functions
- **IP functions**: https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/ip-functions
- **Spatial functions**: https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/spatial-functions
- **Search functions**: https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/search-functions
- **Type conversion functions**: https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/type-conversion-functions
- **Multivalue functions**: https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/mv-functions
- **Dense vector functions**: https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/dense-vector-functions
- **Time series aggregation functions**: https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/time-series-aggregation-functions

## Elasticsearch APIs used by this CLI

- **ES|QL query**: `POST /_query` — executes an ES|QL query
- **Cluster info**: `GET /` — ping / version check
- **List indices**: `GET /_cat/indices?format=json` — index metadata
- **Data streams**: `GET /_data_stream` — list data streams
- **Field mappings**: `GET /<index>/_mapping` — index field mappings
- **Resolve index**: `GET /_resolve/index/<pattern>` — unified view of indices, aliases, data streams
