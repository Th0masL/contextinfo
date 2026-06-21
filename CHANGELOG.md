# Changelog

All notable changes to this project are documented in this file. The format is
loosely based on [Keep a Changelog](https://keepachangelog.com/), and the project
follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- `git.remote` now has embedded credentials stripped (CI checkouts such as
  GitLab embed a token in the origin URL) to avoid leaking it into output/state.
- `CIInfo` gained `actor`, `event`, `repository`, `workflow`, and `server_url`
  (populated for GitHub Actions and GitLab CI).

### Fixed

- `git.branch` is no longer set to the tag name on tag/release events. In CI's
  detached-HEAD checkout the fallback used a ref-name variable that holds the tag
  on tag events; it now uses ref-type-aware fallbacks (`GITHUB_REF_TYPE=branch`,
  GitLab `CI_COMMIT_BRANCH`), leaving `git.branch` empty for tags (`git.tag`
  still carries the tag).

### Added

- Initial `contextinfo` library (`pkg/contextinfo`) exposing `Detect() Info`.
- CI/CD detection for GitHub Actions and GitLab CI (the platforms whose
  environments have been verified), plus a generic `CI=true` → `unknown` fallback
  and a `local` default.
- Git context detection: commit, branch (with CI fallback when detached), tag,
  dirty state, and origin remote.
- Runtime detection: OS, arch, hostname.
- `contextinfo` CLI with formats: `envvar` (**default** — shell `NAME=value`
  lines), `json` (nested), `json-flat`, `text`, `tfvars` (HCL), `tfvars-json`.
- `--prefix` flag for the flat/envvar formats (default: no prefix), e.g.
  `--format=envvar --prefix TF_VAR_` → `TF_VAR_git_commit=...` for Terraform.
- Rich `--help` with a description, the flags, the format list, and examples.
- `Info.EnvVars(prefix)` / `Info.FlatJSON(prefix)` / `Info.TFVarsHCL(prefix)` /
  `Info.TFVarsJSON(prefix)` library methods (HCL/shell output is safely escaped).
- GoReleaser configuration and CI/release GitHub Actions workflows.
