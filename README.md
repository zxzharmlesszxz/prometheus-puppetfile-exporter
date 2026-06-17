# prometheus-puppetfile-exporter

Small Prometheus exporter that reads a `Puppetfile` and exposes pinned module versions.

It does not call Puppet Forge, GitLab, or Git directly. The exporter is intentionally narrow: it only reports what is declared in `Puppetfile`.
The exporter also exposes source-health metrics for the `Puppetfile` itself, so Prometheus can distinguish a readable, parseable file from a missing or malformed one.
Unsupported or incomplete module declarations are skipped and reported through diagnostics metrics instead of failing the entire scrape.

## Inputs

The exporter reads one local `Puppetfile`.

## Metrics

The exporter exposes one metric per supported pinned module declaration plus source-health and diagnostics metrics for the file itself.

Example:

```code
puppetfile_module_info{module="puppetlabs/apt",owner="puppetlabs",name="apt",current_version="11.2.0",type="forge"} 1
```

Important metric groups:

- `puppetfile_up`
- `puppetfile_valid`
- `puppetfile_mtime_seconds`
- `puppetfile_module_info`
- `puppetfile_modules_discovered`
- `puppetfile_unsupported_modules_count`
- `puppetfile_incomplete_modules_count`
- `puppetfile_read_errors_total`
- `puppetfile_parse_errors_total`
- `puppetfile_scrape_duration_seconds`

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
  snapshot gathering, and parser.

Common exporter bootstrap, HTTP handlers, registry wiring, landing page, health endpoint, logging, and version metadata are provided by `github.com/zxzharmlesszxz/prometheus-exporter-framework`.

## Local Run

Run against the bundled sample file:

```bash
make build
./dist/prometheus-puppetfile-exporter \
  --puppetfile.puppetfile-path=./testdata/Puppetfile
```

`pprof` is disabled by default. Enable it explicitly only when you need runtime profiling:

```bash
./dist/prometheus-puppetfile-exporter \
  --puppetfile.puppetfile-path=./testdata/Puppetfile \
  --web.enable-pprof
```

## Local Docker Compose

```bash
make compose
```

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

```yaml
# single Puppetfile
puppetfile_path: /etc/puppetlabs/code/environments/production/Puppetfile
```

```yaml
# multiple explicit files
puppetfile_path: /etc/puppetlabs/code/environments/production/Puppetfile,/etc/puppetlabs/code/environments/development/Puppetfile
```

```yaml
# all environment Puppetfiles with a glob
puppetfile_path: /etc/puppetlabs/code/environments/*/Puppetfile
```

```yaml
# comma-separated glob plus explicit file
puppetfile_path: /etc/puppetlabs/code/environments/*/Puppetfile,/opt/puppetlabs/extra/Puppetfile
```

## Custom Puppetfile

Use your own file either by editing `configs.exporter_puppetfile` in [`docker-compose.yml`](docker-compose.yml), by running the image directly with explicit flags, or by running locally:

```bash
docker run --rm -p 9820:9820 \
  -v "$PWD/my-Puppetfile:/Puppetfile:ro" \
  prometheus-puppetfile-exporter \
  --puppetfile.puppetfile-path=/Puppetfile
```

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

Run the local Go checks with Go 1.26+:

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
They cover exporter scrape availability, framework collection failure/staleness,
Puppetfile source health, module inventory problems, file discovery, bad module
status, and read/parse error counter increases.

## Grafana Dashboard

Adapted Grafana dashboard JSON for this exporter lives in [`examples/grafana/prometheus-puppetfile-exporter.json`](examples/grafana/prometheus-puppetfile-exporter.json).

## CI

GitHub Actions CI lives in [`.github/workflows/ci.yml`](.github/workflows/ci.yml). It runs the Go checks in `golang:1.26.3-alpine`, validates examples, smoke-tests the Docker image, publishes release archives on tags, and pushes tagged images to GHCR.

The legacy GitLab definition remains in [`.gitlab-ci.yml`](.gitlab-ci.yml).
