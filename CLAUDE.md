# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Prometheus exporter for HBase, written in Go. It scrapes HBase JMX endpoints (HMaster and RegionServer) over HTTP and re-exposes the metrics on a `/metrics` page for Prometheus to scrape. Supports region-level metrics (per namespace/table/region).

## Build & Run

Go-modules project (module path `github.com/daiwei233/hbase_exporter`). Builds with a stock Go 1.23+ toolchain.

```sh
make build          # → ./hbase_exporter
make test           # go test ./...
make docker         # build the container image
```

Run as a master-scraping instance:

```sh
./hbase_exporter --web.listen-address=":9003" \
  --hbase.master.uri="http://localhost:60010/jmx" --hbase.master
```

Run as a regionserver-scraping instance:

```sh
./hbase_exporter --web.listen-address=":9003" \
  --hbase.regionserver.uri="http://localhost:60030/jmx"
```

Flags (all also via env vars, see `hbase_exporter.go`): `web.listen-address` (`:9115`), `web.telemetry-path` (`/metrics`), `hbase.master.uri`, `hbase.regionserver.uri`, `hbase.master` (bool), `log.level`, `log.format`, `log.output`.

Releases are produced by goreleaser (`.goreleaser.yaml`) on tag push (`.github/workflows/release.yaml`); CI runs `go vet` + `go test -race` + build on push/PR (`.github/workflows/ci.yaml`).

## Architecture

`main` (`hbase_exporter.go`, package `main`) parses flags with `kingpin`, builds a `go-kit` logger, then registers Prometheus collectors based on the `--hbase.master` flag and serves `/metrics` via the default `http.ServeMux`:

- `--hbase.master=true`  → registers `Jvm` + `MasterServer` (scrapes the master JMX URI)
- otherwise → registers `Jvm` + `RsServer` + `RsRegion` (scrapes the regionserver JMX URI)

Plus a `hbase_exporter_build_info` gauge exposing build metadata (replaces the dropped `version.NewCollector`). So **one process instance collects for exactly one role**; monitoring a full cluster means running one master instance plus one regionserver instance per RS node. The `--hbase.master` flag is the single role switch.

### Collectors (`collector/` package)

Each collector implements `prometheus.Collector` (`Describe` + `Collect`) and carries the standard three instrumentation metrics: `…_up`, `…_total_scrapes`, `…_json_parse_failures`. The fetch path is shared: `collector.go`'s `jmxCollector[T]` (generic) issues the JMX request through a shared `http.Client` with a 30s timeout, reads `beans[0]` with `gjson`, then `json.Unmarshal`s it into the response struct `T`.

| Collector | File | Subsystem | JMX bean | Role |
| --- | --- | --- | --- | --- |
| `Jvm` | `jvm.go` (`jvm_response.go`) | `jvm` | `JvmMetrics` | both |
| `MasterServer` | `master_server.go` (`master_server_response.go`) | `server` | `Master,sub=Server` | master only |
| `RsServer` | `rs_server.go` (`rs_server_response.go`) | `server` | `RegionServer,sub=Server` | regionserver only |
| `RsRegion` | `rs_region.go` | `region` | `RegionServer,sub=Regions` | regionserver only |

`Jvm`, `MasterServer` and `RsServer` are thin wrappers over `jmxCollector[T]`: each just declares its response type and a `[]metricDef[T]` list, then embeds the base, which supplies `Describe`/`Collect`/`fetchAndDecode`. The `gauge[T]` helper (in `jvm.go`) builds a labelled gauge `metricDef` over the `(host, role)` label set; `hostRoleLabels` lower-cases the `role`.

`RsRegion` does not fit the static pattern — region metrics have dynamic per-region label sets and an unknown-at-build-time field set. Its `Collect` walks the JMX `beans[0]` map for keys starting with `Namespace`, parses each with `utils.SplitHBaseRegionStr`, maps the JMX field suffix to an exporter metric name via the `regionMetric` table, and emits a `MustNewConstMetric` against a pre-built `*prometheus.Desc` (`descs` map). All four collectors share the `namespace = "hbase"` constant and the `subsystem{Jvm,Server,Region}` constants declared in `collector.go` — do not redeclare.

All metric names are built with `prometheus.BuildFQName(namespace, subsystem, name)`. To add a static metric: add a struct field to the response type and a `gauge(...)` entry. To add a region metric: add the JMX field → exporter-name mapping to `regionMetric`.

`utils/string.go::SplitHBaseRegionStr` splits a JMX key like `Namespace_n1_table_t1_region_r1_metric_storeCount` into `[n1, t1, r1, storeCount]` (namespace, table, region, field) and returns an error instead of panicking on malformed input.

### Notes

- Region `role` label comes from JMX `tag.Context` (e.g. `sub`); the server/JVM collectors additionally `strings.ToLower` it via `hostRoleLabels`. Region collector leaves it as-is to match origin.
- The README's metric tables are the source of truth for exported names. The repo targets HBase 1.2.x JMX fields.
