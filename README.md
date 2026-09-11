# prometheus-puppetfile-exporter

Small Prometheus exporter that reads a `Puppetfile` and exposes pinned module versions.

It does not call Puppet Forge, GitLab, or Git directly. The exporter is intentionally narrow: it only reports what is declared in `Puppetfile`.
The exporter also exposes source-health metrics for the `Puppetfile` itself, so Prometheus can distinguish a readable, parseable file from a missing or malformed one.
Unsupported or incomplete module declarations are skipped and reported through diagnostics metrics instead of failing the entire scrape.

## Inputs

The exporter reads one or more local `Puppetfile` files. The configured value
can contain explicit paths, glob patterns, or a comma-separated mix of both.

## Metrics

The exporter exposes one metric per supported pinned module declaration plus source-health and diagnostics metrics for the file itself.

Example:

```code
puppetfile_module_info{module="puppetlabs/apt",owner="puppetlabs",name="apt",current_version="11.2.0",type="forge"} 1
```

Important metric groups:

- `puppetfile_exporter_*` for framework/exporter runtime and collection health
- `puppetfile_up`
- `puppetfile_valid`
- `puppetfile_source_data_available`
- `puppetfile_source_data_stale`
- `puppetfile_source_consecutive_failures`
- `puppetfile_source_last_success_timestamp_seconds`
- `puppetfile_mtime_seconds`
- `puppetfile_module_info`
- `puppetfile_modules_discovered`
- `puppetfile_unsupported_modules_count`
- `puppetfile_incomplete_modules_count`
- `puppetfile_read_errors_total`
- `puppetfile_parse_errors_total`
- `puppetfile_scrape_duration_seconds`
- `scrape_*` and `up` for Prometheus-side scrape health

The exporter emits `puppetfile_exporter_*` and `puppetfile_*`. Prometheus adds
the target-level `scrape_*` and `up` series when it scrapes the exporter.

Per Puppetfile path, the exporter tracks the last valid source refresh. These
metrics keep current read/parse failures distinct from the availability and
freshness of previously valid data. Cached data is not substituted into the
current module metrics.

## Input Semantics

The parser is intentionally limited to the official declarative forms that are useful for version export.

Supported and exported:

- Forge modules pinned by explicit version
- Git modules pinned by `:tag`, `:branch`, `:ref`, or `:commit`
- Both hashrocket syntax (`:git => '...'`) and Ruby 1.9 syntax (`git: '...'`)
- Quoted string values with either single or double quotes
- Symbol branch/ref values such as `:branch => :control_branch`

Recognized but reported as incomplete:

- Bare Forge declarations such as `mod 'puppetlabs/apache'`
- Forge declarations pinned to `:latest`
- Git declarations that specify `:git` but no `:tag`, `:branch`, `:ref`, or `:commit`

Recognized and ignored as auxiliary official options:

- `moduledir`
- `forge`
- `:default_branch`
- `:install_path`
- `:exclude_spec`

Still intentionally unsupported:

- Local path modules such as `:path`
- Arbitrary Ruby code or highly dynamic `Puppetfile` constructs

The shape of `puppetfile_module_info` is intentionally close to the metrics exposed by our Forge service, so Grafana and PromQL can compare `current_version` from `Puppetfile` with `latest_version` from Forge using shared labels.

Labels:

- `module`: module identifier from `Puppetfile`
- `owner`: parsed module owner or namespace; for module names without `/` this is a best-effort heuristic based on the first `-`
- `name`: parsed module name; for module names without `/` this is a best-effort heuristic based on the first `-`
- `current_version`: pinned version from `Puppetfile`
- `type`: `forge`, `git+tag`, `git+branch`, `git+ref`, or `git+commit`

## Project Layout

- `cmd` contains the minimal process entrypoint.
- `internal/exporter` contains the thin scaffold-owned adapter that creates the Puppetfile feature and delegates injected project metadata to the framework.
- `internal/puppetfile` contains the Puppetfile feature spec, collector metrics,
  and snapshot integration.
- `internal/puppetfilecheck` contains path-level reads, parsing, diagnostics, and
  last-known-good source state.

Common exporter bootstrap, HTTP handlers, registry wiring, landing page, health endpoint, logging, and version metadata are provided by `github.com/zxzharmlesszxz/prometheus-exporter-framework`.

## Local Run

Run against the bundled sample file:

```bash
make build
./dist/prometheus-puppetfile-exporter \
  --puppetfile.puppetfile-path=./smoke/testdata/Puppetfile
```

`pprof` is disabled by default. Enable it explicitly only when you need runtime profiling:

```bash
./dist/prometheus-puppetfile-exporter \
  --puppetfile.puppetfile-path=./smoke/testdata/Puppetfile \
  --web.enable-pprof
```

## Local Docker Compose

```bash
make compose
```

Override `COMPOSE_EXPORTER_HOST_PORT` to change only the host-side exporter
port. The container, its health check, and the Prometheus scrape target keep
using port `9820`.

This starts:

- exporter on `http://localhost:9820/metrics`
- Prometheus on `http://localhost:9090`
- Grafana on `http://localhost:3000` with `admin` / `admin`

Then inspect metrics directly:

```bash
curl http://localhost:9820/metrics
```

Prometheus scrapes the exporter automatically, and Grafana provisions both the Prometheus datasource and the dashboard from `examples/grafana/prometheus-puppetfile-exporter.json`.
The local Prometheus also loads example alert rules from `examples/prometheus/prometheus-puppetfile-exporter.yml`.

## Docker Image Defaults

The binary defaults to:

- `--web.listen-address=:9820`
- `--puppetfile.config-file=/etc/prometheus/prometheus-puppetfile-exporter.yml` when the file exists
- `--puppetfile.puppetfile-path=/etc/puppetlabs/code/environments/production/Puppetfile`

The image does not bundle a `Puppetfile`; you must mount one, provide it through the config file, or override `--puppetfile.puppetfile-path`.
The local `docker-compose.yml` keeps the same `:9820` listen address and injects a bundled sample `Puppetfile` through Compose `configs`.

## Configuration Example

The YAML config file accepts one Puppetfile-specific key:

```yaml
puppetfile_path: /etc/puppetlabs/code/environments/production/Puppetfile
```

It can point at one file, several explicit files, a glob, or a comma-separated
mix of globs and explicit files:

After glob expansion, duplicate paths are removed while preserving the order
of their first appearance.

```yaml
# single Puppetfile
puppetfile_path: /etc/puppetlabs/code/environments/production/Puppetfile
```

```yaml
# multiple explicit files
puppetfile_path: /envs/production/Puppetfile,/envs/testing/Puppetfile
```

```yaml
# all environment Puppetfiles with a glob
puppetfile_path: /etc/puppetlabs/code/environments/*/Puppetfile
```

```yaml
# glob plus explicit file
puppetfile_path: /etc/puppetlabs/code/environments/*/Puppetfile,/opt/extra/Puppetfile
```

## Custom Puppetfile

Use your own file either by editing `configs.exporter_puppetfile` in [`docker-compose.yml`](docker-compose.yml), by running the image directly with explicit flags, or by running locally:

```bash
docker run --rm -p 127.0.0.1:9820:9820 \
  -v "$PWD/my-Puppetfile:/Puppetfile:ro" \
  prometheus-puppetfile-exporter \
  --puppetfile.puppetfile-path=/Puppetfile
```

## Build Configuration

`Makefile.mk` contains the scaffold-owned exporter identity and contract.
Overridable tooling, build metadata, image tags, and Compose defaults live in
`Makefile.defaults.mk`. Put persistent machine-local overrides in the ignored
`Makefile.local`; do not edit the rendered contract for local configuration.

## Logging

The exporter uses `promslog` with structured `slog` output.

Useful flags:

```bash
./dist/prometheus-puppetfile-exporter \
  --puppetfile.puppetfile-path=./testdata/Puppetfile \
  --log.level=debug \
  --log.format=json \
  --web.enable-pprof
```

At `info` level it logs startup and runtime configuration.
At `warn` level it reports an empty `Puppetfile`, one that yields no supported modules, and any skipped unsupported or incomplete module declarations.
Repeated refreshes with the same warning state do not emit duplicate warnings until that state changes.
At `error` level it reports `Puppetfile` read failures and server startup/runtime failures.
At `debug` level it logs successful collection with the number of exported modules.

## Tests

Run the local Go checks with Go 1.27.1+:

```bash
make go-check
```

Validate the shipped Prometheus and Compose examples:

```bash
make examples-check
```

Build and smoke-test the Docker image:

```bash
make docker-smoke
```

Build release archives and smoke-test the native archive:

```bash
make release-smoke
```

## Scaffold-Owned Go Files

Go files named `scaffold_*.go` are generated contract glue and should stay identical to the scaffold output. Add Puppetfile-specific behavior in adjacent non-scaffold files such as `internal/puppetfile/*_ext.go` and `internal/puppetfilecheck`.

## Metrics Reference

The full metric contract lives in [`METRICS.md`](METRICS.md).

## Architecture

The high-level design is documented in [`ARCHITECTURE.md`](ARCHITECTURE.md).

## Alert Rules

Example Prometheus alert rules live in [`examples/prometheus/prometheus-puppetfile-exporter.yml`](examples/prometheus/prometheus-puppetfile-exporter.yml).
They use `job=~"^(prometheus[_-])?puppetfile[_-]exporter$"` and cover exporter
scrape availability, framework collection failure/staleness, unreadable and
parse-invalid Puppetfiles, module source/domain problems, and read/parse error
counter increases. A per-path alert also reports when the last valid source
data remains stale after repeated failures.

## Grafana Dashboard

Adapted Grafana dashboard JSON for this exporter lives in [`examples/grafana/prometheus-puppetfile-exporter.json`](examples/grafana/prometheus-puppetfile-exporter.json).
It uses Grafana dashboard schema v2: domain status stats, last-valid source
availability, and the Puppetfile
module table lead the Overview tab, Source Health stays on that tab, exporter
collection graphs use two columns, and the collapsed historical row uses three.

## CI

GitHub Actions CI lives in [`.github/workflows/ci.yml`](.github/workflows/ci.yml)
and delegates checks, example validation, Docker smoke tests, release archives,
and GHCR publishing to the framework reusable workflow at `v0.7.0` using Go
1.27.1.

The legacy GitLab definition remains in [`.gitlab-ci.yml`](.gitlab-ci.yml).
