# Metrics

## Health Metrics

### `puppetfile_up`

- Type: gauge
- Value:
  - `1` if `Puppetfile` was readable during the last refresh
  - `0` on read failure
- Labels: `path`
- Notes: source availability metric

### `puppetfile_valid`

- Type: gauge
- Value:
  - `1` if `Puppetfile` was readable and had no unsupported or incomplete module declarations
  - `0` on read failure or when unsupported/incomplete module declarations were found
- Labels: `path`
- Notes: source validity metric for dashboards and alerts

### `puppetfile_mtime_seconds`

- Type: gauge
- Value: Unix mtime of `Puppetfile`, or `0` if unavailable
- Labels: `path`
- Notes: source freshness metric

### `puppetfile_modules_discovered`

- Type: gauge
- Value: number of supported pinned modules discovered during the last refresh
- Labels: `path`
- Notes: `0` is valid for an empty `Puppetfile` or one without supported modules

### `puppetfile_exporter_last_collection_success`

- Type: gauge
- Value:
  - `1` if the latest background refresh completed successfully
  - `0` if the latest background refresh failed
- Labels: none

### `puppetfile_exporter_last_collection_timestamp_seconds`

- Type: gauge
- Value: Unix timestamp of the last background refresh attempt
- Labels: none

### `puppetfile_exporter_last_successful_collection_timestamp_seconds`

- Type: gauge
- Value: Unix timestamp of the last successful background refresh
- Labels: none

## Business Metrics

### `puppetfile_module_info`

- Type: gauge
- Value: always `1`
- Labels:
  - `path`
  - `module`
  - `owner`
  - `name`
  - `current_version`
  - `type`
- Notes:
  - for module identifiers without `/`, `owner` and `name` are derived heuristically by splitting on the first `-`

### `puppetfile_files_monitored`

- Type: gauge
- Value: number of Puppetfile paths configured and monitored (after glob expansion)
- Labels: none
- Notes: `0` means no paths were configured; useful for alerting on misconfigured glob patterns

### `puppetfile_declared_modules_total`

- Type: gauge
- Value: total number of declared modules (supported + unsupported + incomplete) in Puppetfile
- Labels: `path`
- Notes: quick view of Puppetfile size alongside the per-category counts

### `puppetfile_module_status`

- Type: gauge
- Value:
  - `1` if the module declaration is supported and well-formed
  - `0` if the module declaration is unsupported or incomplete
- Labels: `path`, `module`
- Notes: unlike `puppetfile_module_info` (only OK modules), this includes every declaration. Use `puppetfile_module_status == 0` to find broken modules.

Supported module kinds:

- `type="forge"` for pinned Forge modules
- `type="git+tag"` for Git modules pinned by `:tag`
- `type="git+branch"` for Git modules pinned by `:branch`
- `type="git+ref"` for Git modules pinned by `:ref`
- `type="git+commit"` for Git modules pinned by `:commit`

## Diagnostics Metrics

### `puppetfile_unsupported_modules_count`

- Type: gauge
- Value: number of unsupported module declarations skipped during the last refresh
- Labels: `path`
- Notes: examples include local path modules or other declaration forms that this exporter does not model

### `puppetfile_incomplete_modules_count`

- Type: gauge
- Value: number of incomplete module declarations skipped during the last refresh
- Labels: `path`
- Notes:
  - examples include bare Forge declarations, `:latest`, or Git declarations without `:tag`, `:branch`, `:ref`, or `:commit`
  - auxiliary official options such as `:default_branch`, `:install_path`, and `:exclude_spec` do not make a module incomplete on their own when a supported pin is present

### `puppetfile_read_errors_total`

- Type: counter
- Value: total number of `Puppetfile` read errors
- Labels: `path`
- Notes: monotonic read/open error counter

### `puppetfile_parse_errors_total`

- Type: counter
- Value: total number of Puppetfile refreshes that found unsupported or incomplete module declarations
- Labels: `path`
- Notes: monotonic validity counter; it increments once per invalid refresh

### `puppetfile_scrape_duration_seconds`

- Type: gauge
- Value: duration of the most recent Puppetfile refresh
- Labels: `path`
- Notes: preserved metric name from the pre-refresh collector implementation
