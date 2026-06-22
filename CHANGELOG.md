# Changelog

All notable changes to this project are documented in this file. The format is
loosely based on [Keep a Changelog](https://keepachangelog.com/), and the project
follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `contextinfo` library (`pkg/contextinfo`) exposing `Detect() Info`, and the
  `contextinfo` CLI.
- A single flat `Info` set, resolved local-first — git/OS values are primary and
  CI variables augment them, so it works the same in and out of CI. Fields:
  `git_branch`, `git_commit_sha`, `git_commit_sha_short`, `git_tag`, `git_dirty`,
  `git_checksum`, `git_repo_url`, `git_repository`, `actor`, `event`,
  `ci_platform`, `ci_build_url`, `ci_build_number`, `ci_workflow`,
  `runtime_hostname`.
  - `git_repository` / `git_repo_url` are derived from the git remote (ssh→https,
    embedded credentials stripped); `actor` falls back to the OS user; `event`
    defaults to `manual`.
  - `git_branch` is branch-only — on a tag/detached checkout it stays empty and
    `git_tag` carries the tag (the CI ref hints are ref-type-aware, so a tag is
    never mislabeled as a branch).
  - `git_checksum` is a SHA-256 over the non-ignored files in the working
    directory (`git ls-files --cached --others --exclude-standard`, sorted) — a
    content fingerprint independent of commit history. Symlinks are followed and
    the target's content is hashed (for Terraform stacks symlinking shared files
    from parent folders). Computed by default; skip with the `--no-checksum` flag
    or `contextinfo.WithoutChecksum()`.
- CI/CD detection for GitHub Actions, GitLab CI, and CircleCI (the platforms
  whose environments have been verified), plus a generic `CI=true` → `unknown`
  fallback and a `""` (local) default. Per-provider detection lives in
  `github.go` / `gitlab.go` / `circleci.go` behind an env-injectable core, with
  golden tests over committed real CI environment dumps in
  `pkg/contextinfo/testdata/env`. CircleCI has no native event variable, so its
  `event` is derived (`tag` when `CIRCLE_TAG` is set, else `push`).
- CLI formats: `envvar` (**default** — shell `NAME=value` lines), `json` (flat),
  `text`, `tfvars` (HCL), `tfvars-json`.
- `--prefix` flag for the `envvar` / `tfvars` / `tfvars-json` formats (default: no
  prefix), e.g. `--format=envvar --prefix TF_VAR_` → `TF_VAR_git_commit_sha=...`.
- Rich `--help` with a description, the flags, the format list, and examples.
- `Info.EnvVars(prefix)` / `Info.FlatJSON(prefix)` / `Info.TFVarsHCL(prefix)` /
  `Info.TFVarsJSON(prefix)` library methods (HCL/shell output is safely escaped).
- GoReleaser configuration and CI/release GitHub Actions workflows.
