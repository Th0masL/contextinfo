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
  `files_checksum`, `git_repo_url`, `git_repository`, `actor`, `event`,
  `ci_platform`, `ci_build_url`, `ci_build_number`, `ci_workflow`,
  `runtime_hostname`.
  - `git_repository` / `git_repo_url` are derived from the git remote (ssh→https,
    embedded credentials stripped); `actor` falls back to the OS user.
  - `event` is normalized to a cross-provider vocabulary — `push`, `tag`,
    `pull_request`, `release`, `schedule`, `manual` — so e.g. a GitHub tag push
    and a GitLab tag pipeline both report `tag`; uncommon platform triggers pass
    through their raw value, and it defaults to `manual` outside CI.
  - `git_branch` is branch-only — on a tag/detached checkout it stays empty and
    `git_tag` carries the tag (the CI ref hints are ref-type-aware, so a tag is
    never mislabeled as a branch).
  - `files_checksum` is a SHA-256 over the non-ignored files in the working
    directory — a content fingerprint independent of commit history. It is
    byte-for-byte reproducible with `git ls-files -z --cached --others
    --exclude-standard | LC_ALL=C sort -z | xargs -0 -r sha256sum | sha256sum`:
    each file's content is hashed (symlinks followed), and unreadable/non-regular
    paths are skipped as `sha256sum` skips them. Computed by default; skip with
    the `--no-files-checksum` flag or `contextinfo.WithoutFilesChecksum()`.
- CI/CD detection for GitHub Actions, GitLab CI, and CircleCI (the platforms
  whose environments have been verified), plus a generic `CI=true` → `unknown`
  fallback and a `""` (local) default. Per-provider detection lives in
  `github.go` / `gitlab.go` / `circleci.go` behind an env-injectable core, with
  golden tests over committed real CI environment dumps in
  `pkg/contextinfo/testdata/env`. CircleCI has no native event variable, so its
  `event` is derived from `CIRCLE_TAG` / `CIRCLE_PULL_REQUEST` / `CIRCLE_BRANCH`.
- CLI formats: `envvar` (**default** — shell `NAME=value` lines), `json` (flat),
  `text`, `tfvars` (HCL). A flat JSON object is valid `.tfvars.json`, so `json`
  doubles as the Terraform-JSON output.
- `--prefix` flag for the `envvar` / `json` / `tfvars` formats (default: no
  prefix), e.g. `--format=envvar --prefix TF_VAR_` → `TF_VAR_git_commit_sha=...`.
- `contextinfo.WithDir(dir)` library option and `--dir` CLI flag to inspect a
  directory other than the current one. `Detect` holds no global state, so it is
  safe to call concurrently for different directories.
- `contextinfo.WithExplain()` / `--explain`: emits a `<field>_explained` companion
  after each field naming where the value came from (env var(s) or git command),
  in every format. The notes name variables and commands, not their contents, so
  they never expose secrets. Adds `Info.Text()` for the `text` format.
- Rich `--help` with a description, the flags, the format list, and examples.
- `Info.EnvVars(prefix)` / `Info.FlatJSON(prefix)` / `Info.TFVarsHCL(prefix)` /
  `Info.Text()` library methods (HCL/shell output is safely escaped).
- GoReleaser configuration and CI/release GitHub Actions workflows.
