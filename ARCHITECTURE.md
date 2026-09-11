# Architecture

## Overview

`prometheus-puppetfile-exporter` reads a local `Puppetfile` and exposes pinned module declarations as Prometheus metrics over HTTP.

The exporter does not talk to Puppet Forge, Git repositories, Puppet Server, or PuppetDB. It is intentionally file-backed and reports only what is declared in the local `Puppetfile`.

## Package Layout

- `cmd`
  Minimal process entrypoint in scaffold-owned `scaffold_main.go`.
- `internal/exporter`
  Thin scaffold-owned exporter adapter that creates the Puppetfile feature and delegates injected project metadata to the framework.
- `internal/puppetfile`
  Puppetfile feature package. `scaffold_feature.go` implements the stable
  `featurekit.FeatureContract` bridge supported by the current framework
  release and wires config-file flags, runtime config, collector construction,
  metrics, snapshot status, and smoke behavior through Puppetfile hook
  functions. Scaffold-owned files use the `scaffold_*.go` suffix;
  Puppetfile-specific defaults and hook functions live in adjacent non-scaffold
  feature files.
  `snapshot_types.go` owns the Puppetfile-specific snapshot wrapper around
  `internal/puppetfilecheck.Snapshot`.
- `internal/puppetfilecheck`
  Puppetfile engine: file reading, parser, module declaration types,
  diagnostics, and snapshot-backed checker.

Common exporter bootstrap, standard flags, logging, version metadata, Prometheus registry setup, HTTP handlers, landing page, health endpoint, and optional pprof endpoints are provided by the sibling `prometheus-exporter-framework` module.

## Data Flow

1. The process starts in `cmd`.
2. `prometheus-exporter-framework` registers common flags and delegates Puppetfile-specific behavior through extension hooks.
3. Runtime flags determine the explicit paths and glob patterns used to find
   Puppetfiles, the refresh interval, and HTTP settings.
4. `internal/exporter` creates the Puppetfile feature through
   `puppetfile.NewFeature(...)` and framework-injected feature metadata.
5. The framework builds a Prometheus registry and builds a feature snapshotter and feature metrics from the extension-backed spec.
6. The feature snapshotter expands comma-separated path/glob inputs and
   delegates each Puppetfile read and parse to `internal/puppetfilecheck`.
7. The snapshot collector refreshes the combined Puppetfile state in the background.
8. Prometheus scrapes expose the latest successful or failed snapshot.
9. Per-path `featurekit.LastKnownGood` state records the last readable and valid
   Puppetfile refresh separately from current source health. It drives
   availability, consecutive-failure, last-success, and staleness metrics but
   does not replace current module data after a failed refresh.
10. Supported module declarations are exported as `puppetfile_module_info`.
11. Unsupported or incomplete declarations are skipped and counted in
    diagnostics metrics.
12. If the file cannot be read, `puppetfile_up` is set to `0` and current
    business metrics are omitted. Unsupported or incomplete declarations keep
    `puppetfile_up` at `1` and set `puppetfile_valid` to `0`.

## Container Defaults

The binary defaults to:

- `--web.listen-address=:9820`
- default `--puppetfile.puppetfile-path=/etc/puppetlabs/code/environments/production/Puppetfile`

The image itself does not include a bundled `Puppetfile`; callers are expected to mount one or override the flag.
The local `docker-compose.yml` keeps the same `:9820` listen address and injects a sample `Puppetfile` through Compose `configs`.

## Source Semantics

The exporter currently models:

- Forge modules pinned by version
- Git modules pinned by `:tag`
- Git modules pinned by `:branch`
- Git modules pinned by `:ref`
- Git modules pinned by `:commit`
- Both hashrocket syntax and Ruby 1.9 key syntax for Git options
- Symbol values such as `:branch => :control_branch`

The exporter intentionally does not resolve remote state. It reports declared versions or refs only.

Declarations that are syntactically present but not modeled by the exporter, such as local path modules, are skipped and surfaced through diagnostics metrics instead of failing the entire scrape.
Official but unpinned declarations, such as bare Forge entries or `:latest`, are treated as incomplete because this exporter is intentionally focused on stable current-version labels.
Auxiliary official options such as `:default_branch`, `:install_path`, and `:exclude_spec` are tolerated but do not affect exported version labels.

## Metric Semantics

### Health Metrics

- `puppetfile_up`
  `1` means the file was read and parsed successfully.
  `0` means the file could not be read or parsed.

- `puppetfile_mtime_seconds`
  Unix timestamp of the `Puppetfile` modification time when available.

### Business Metrics

- `puppetfile_module_info`
  Exposes one metric per supported pinned module declaration.

- `puppetfile_modules_discovered`
  Number of supported pinned modules discovered during the most recent refresh.

- `puppetfile_exporter_last_collection_success`
  `1` means the latest background refresh completed successfully.
  `0` means the latest background refresh failed.

- `puppetfile_exporter_last_collection_timestamp_seconds`
  Unix timestamp of the latest background refresh attempt.

- `puppetfile_exporter_last_successful_collection_timestamp_seconds`
  Unix timestamp of the latest successful background refresh.

- `puppetfile_exporter_collection_duration_seconds`
  Framework-owned histogram of background collection duration.

- `puppetfile_exporter_build_info`
  Framework-owned build and runtime metadata.

### Diagnostics Metrics

- `puppetfile_unsupported_modules_count`
  Number of unsupported module declarations skipped during the most recent refresh.

- `puppetfile_incomplete_modules_count`
  Number of incomplete module declarations skipped during the most recent refresh.

- `puppetfile_read_errors_total`
  Monotonic counter of read/open errors.

- `puppetfile_scrape_duration_seconds`
  Duration of the most recent `Puppetfile` refresh.

For module identifiers without `/`, the exporter derives `owner` and `name` heuristically by splitting on the first `-`.

## Failure Semantics

- If `Puppetfile` cannot be read or parsed, `puppetfile_up = 0` and business metrics are omitted.
- Unsupported and incomplete declarations do not fail the scrape; they are surfaced through diagnostics metrics.
- `/healthz` reflects process health, not whether the current `Puppetfile` contents are valid for export.

Prometheus-side scrape health is represented separately by `scrape_*` metrics
and `up`; these are produced by Prometheus, not by this exporter.

## Testing Strategy

The project uses:

- unit tests for parser behavior
- collector tests for metric semantics and logging
- scaffold-owned feature integration tests for adapter wiring
- smoke tests generated from the typed Puppetfile feature spec
- fixture-backed manual testing through [docker-compose.yml](docker-compose.yml)

[METRICS.md](METRICS.md) contains the canonical metric contract.
