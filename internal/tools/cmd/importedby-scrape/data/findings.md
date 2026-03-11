# Attribute Slice Findings

## Scope

This note summarizes two data sources used to evaluate short-length specialization
for `go.opentelemetry.io/otel/attribute` slice handling:

1. Downstream source scanning using the local tooling in
   `internal/tools/cmd/importedby-scrape`.
2. OpenTelemetry semantic conventions from
   `/home/tyler/code/semantic-conventions`.

## Downstream Source Scan

Pipeline used:

1. Scrape `pkg.go.dev` imported-by results for `go.opentelemetry.io/otel/attribute`.
2. Collapse importer package paths to repo candidates.
3. Enrich GitHub repos with metadata and build a stratified sample.
4. Shallow-clone the sample.
5. Scan `attribute.BoolSlice`, `IntSlice`, `Int64Slice`, `Float64Slice`, and `StringSlice`
   call sites.

At the time of the latest scan there were 285 completed GitHub clones in:

- `internal/tools/cmd/importedby-scrape/data/repos`

Current scan output:

- `internal/tools/cmd/importedby-scrape/data/slice-usage.csv`

Observed counts from the current sampled corpus:

- `StringSlice`: total `131`, dynamic `121`, literal length `0`: `1`, length `1`: `8`, length `2`: `1`
- `IntSlice`: total `5`, all dynamic
- `Int64Slice`: total `6`, all dynamic
- `BoolSlice`: total `3`, all dynamic
- `Float64Slice`: total `3`, all dynamic

Interpretation:

- Real downstream source code overwhelmingly passes variables, not inline slice literals.
- Simple local alias resolution recovered a small number of literal `StringSlice` calls, mostly
  length `1`, with one observed length `2`.
- Static source scraping alone does not strongly justify a specific production cutoff like `<=3`
  or `<=4`, because most real call sites hide the final length behind runtime values.

## Semantic Convention Slice Inventory

The semantic conventions do define many slice-valued attributes, almost all as `string[]`
or `template[string[]]`.

Representative examples and implied lengths:

### Usually very small

- `browser.brands`
  `model/browser/registry.yaml`
  Example: `[" Not A;Brand 99", "Chromium 99", "Chrome 99"]`
  Expected length: around `2-3`.

- `gen_ai.request.stop_sequences`
  `model/gen-ai/registry.yaml`
  Example: `[forest, lived]`
  Expected length: usually small, often `0-4`.

- `gen_ai.request.encoding_formats`
  `model/gen-ai/registry.yaml`
  Examples: `["base64"]`, `["float", "binary"]`
  Expected length: usually `1-2`.

- `gen_ai.response.finish_reasons`
  `model/gen-ai/registry.yaml`
  Examples: `[stop]`, `[stop, length]`
  Expected length: tied to number of generated choices; often `1`, sometimes a few.

- `messaging.rocketmq.message.keys`
  `docs/attributes-registry/messaging.md`
  Example: `["keyA", "keyB"]`
  Expected length: usually small.

- `user.roles`
  `model/user/registry.yaml`
  Example: `[admin, reporting_user]`
  Expected length: usually small.

- `file.attributes`
  `model/file/registry.yaml`
  Example: `['readonly', 'hidden']`
  Expected length: usually small.

- `tls.client.certificate_chain`, `tls.server.certificate_chain`
  `model/tls/registry.yaml`
  Example: `["MII...", "MI..."]`
  Expected length: usually `1-3`, occasionally higher.

- `tls.client.supported_ciphers`
  `model/tls/registry.yaml`
  Example shows two values, but in practice this can be much larger than `3`.

### Often moderate

- `host.ip`, `host.mac`
  `model/host/registry.yaml`
  Examples show `2`.
  Expected length: number of non-loopback interfaces; commonly `1-4`, sometimes higher.

- `azure.cosmosdb.operation.contacted_regions`
  `model/azure/registry.yaml`
  Example shows `3`.
  Note explicitly says more than one region implies cross-region operation.
  Expected length: often `1`, occasionally a small handful.

- `container.image.tags`, `container.image.repo_digests`, `container.command_args`
  `model/container/registry.yaml`
  Examples show `2-3`.
  Expected length: usually small to moderate.

- `process.command_args`
  `model/process/registry.yaml`
  Example shows `2`.
  The spec also defines `process.args_count`, which implies length matters and may vary widely.
  Expected length: unbounded, but many real commands are still short.

- `aws.log.group.names`, `aws.log.group.arns`, `aws.log.stream.names`, `aws.log.stream.arns`
  `model/aws/registry.yaml`
  Notes explicitly say multiple groups/streams must be supported.
  Expected length: often `1`, sometimes a few.

### Potentially unbounded or transport-driven

- `http.request.header.<key>`, `http.response.header.<key>`
  `model/http/registry.yaml`
  The spec explicitly allows either multiple header values as an array or a single-item array with
  a comma-concatenated string depending on library behavior.
  Expected length: often `1`, sometimes several, occasionally large.

- `rpc.connect_rpc.*.metadata.<key>`, `rpc.grpc.*.metadata.<key>`
  `model/rpc/registry.yaml`
  Examples show one or two values.
  Expected length: usually small per key, but not statically bounded.

- AWS DynamoDB arrays such as:
  - `aws.dynamodb.table_names`
  - `aws.dynamodb.attributes_to_get`
  - `aws.dynamodb.global_secondary_indexes`
  - `aws.dynamodb.local_secondary_indexes`
  - `aws.dynamodb.attribute_definitions`
  - `aws.dynamodb.global_secondary_index_updates`
  `model/aws/registry.yaml`
  Some of these are naturally small; some are request-shape dependent and potentially larger.

## Recommendation

The semantic conventions support keeping a short specialization path. There are many semconv
slice attributes where lengths of `1-3` are common:

- browser brands
- stop sequences
- encoding formats
- finish reasons
- user roles
- file attributes
- certificate chains
- container tags / command args
- host addresses
- contacted regions

At the same time, the conventions also include clearly unbounded or transport-driven arrays:

- HTTP header values
- RPC metadata values
- process command args
- several AWS DynamoDB request/response arrays
- supported ciphers

Practical conclusion:

- A short specialization path is justified.
- Semantic conventions do not support pushing the specialization very far.
- `0..3` remains a defensible cutoff.

Why `0..3` is a reasonable balance:

- It covers the common semconv examples that are explicitly tiny.
- It avoids overfitting to rare larger arrays where reflection fallback is acceptable.
- It matches the local repo literal distribution gathered earlier, where `1-3` dominated.

What this data does **not** prove:

- It does not prove that `0..3` is globally optimal for all production traffic.
- It does not justify claiming that most downstream runtime arrays are length `<=3`, because
  the downstream source scan shows that many call sites are variable-based and their actual
  runtime lengths are not visible statically.
