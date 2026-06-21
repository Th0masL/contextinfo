# Changelog

All notable changes to this project are documented in this file. The format is
loosely based on [Keep a Changelog](https://keepachangelog.com/), and the project
follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Initial `contextinfo` library (`pkg/contextinfo`) exposing `Detect() Info`.
- CI/CD detection for GitHub Actions, GitLab CI, CircleCI, Jenkins, Travis CI,
  and Buildkite, plus a generic `CI=true` fallback and `local` default.
- Git context detection: commit, branch (with CI fallback when detached), tag,
  dirty state, and origin remote.
- Runtime detection: OS, arch, hostname.
- `contextinfo` CLI that prints the detected context as nested JSON, flat JSON
  (`json-flat`), text, or Terraform variables (`tfvars` HCL / `tfvars-json` JSON).
- `--prefix` flag for the flat formats (default: no prefix), e.g.
  `--format=tfvars --prefix TF_VAR_` → `TF_VAR_git_commit`.
- `Info.FlatJSON(prefix)` / `Info.TFVarsHCL(prefix)` / `Info.TFVarsJSON(prefix)`
  library methods that flatten the context (HCL output escapes `${`/`%{`).
- GoReleaser configuration and CI/release GitHub Actions workflows.
