## 1.0.0 (2025-10-16)

### Features

* add github actions to release ssm ([2656de3](https://github.com/andreclaro/ssm/commit/2656de38c34a9ef6d72e5ab7166f6a3bd984c952))
* **app:** add port(s) forward support ([136e4f4](https://github.com/andreclaro/ssm/commit/136e4f497f4acd9fd2c19a2957ee7f159eb42b6a))
* first release ([4dd2863](https://github.com/andreclaro/ssm/commit/4dd28634a85cce997d317bfca60ccbc2331a192c))
* **github:** add pr template ([5794305](https://github.com/andreclaro/ssm/commit/57943058280641612af61f1aada07da09e1fbf27))

### Bug Fixes

* list all running/Online instances ([77edcbd](https://github.com/andreclaro/ssm/commit/77edcbd02b143811b319d623c488b662fa0db87f))

## 0.5.0 (2025-10-16)

### Changed
- FindByName finds an instance by name, preferring reachable instances

## 0.4.0 (2025-10-09)

### Added
- Added pr template (github)

### Fixed
- Fix list issue to allow list all instances "running" and "Online"


## 0.3.0 (2025-10-07)

### Changed
- Pass Ctrl+C to the SSM shell by exec'ing the AWS CLI in `internal/aws/ssm.go`.

## 0.2.0 (2025-10-06)

### Added
- Added port forward (`--forward`) functionality with multi ports support in one line: `ssm server -f 8888:80 -f 8889:89`


## 0.1.0 (2025-10-06)

Discover SSM-managed instances across AWS profiles/regions, cache them locally, and connect via AWS SSM Session Manager.

Basic features:
- Multi-account support via AWS CLI profiles
- Multi-region discovery of EC2 and SSM-managed instances
- Local caching in SQLite at `~/.ssm/database.db`
- Connect via SSM Session Manager
- Simple CLI (`list`, `sync`, connect by name) with shell completion
- Config at `~/.ssm` with `--profile`, `--region`, `--verbose`
- Smart listing: default shows Online SSM-managed instances; `--all` shows all

Advanced features:
- Dynamic region discovery via `DescribeRegions` (with static fallback)
- Performance improvements: batched DB writes and concurrency-limited discovery
- De-duplication across EC2 (i-*) and SSM-managed (mi-*) instances
- First-run auto-setup for profiles and regions
- Quick toggles: `--add-region/--remove-region`, `--add-profile/--remove-profile`
- Stale cleanup using TTL-based pruning

Other changes:
- SSM naming: prefer SSM `Name` over `ComputerName` (fallback) to avoid domain suffixes in listings.
- Config: align default config directory to `~/.ssm` and update help text accordingly.
- Storage: add `SaveOrUpdateBatch` and use it in discovery to reduce transactions.
- Storage: batch tag writes with `CreateInBatches` in `SaveOrUpdate`.
- Storage: remove tag preload in `InstanceRepository.List` to reduce query overhead.
- Tests: ensure in-memory DB is used by assigning `storage.DB = db` in tests.
- Docs: update README Features, splitting into Basic features and Advanced features with detailed bullets.
