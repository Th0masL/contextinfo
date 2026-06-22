# contextinfo

[![CI](https://github.com/Th0masL/contextinfo/actions/workflows/ci.yml/badge.svg)](https://github.com/Th0masL/contextinfo/actions/workflows/ci.yml)

Detect the execution context of a process — **git state**, the **actor / event /
repository** behind the run, and (when present) the **CI/CD platform** — as a
clean Go library and a standalone binary.

Detection is **local-first**: branch, commit, tag, dirty state, repository, and
actor are derived from `git` and the OS, so `contextinfo` behaves the same
whether or not it runs in CI. CI variables only *augment* those values where they
are more authoritative (the branch on a detached-HEAD checkout, the triggering
user, the build URL, …).

`contextinfo` has **no external dependencies** (standard library only) and never
fails: when something can't be detected, the corresponding field is left empty.

## Install

Binary:

```console
go install github.com/Th0masL/contextinfo/cmd/contextinfo@latest
```

Or download a prebuilt binary from the [Releases](https://github.com/Th0masL/contextinfo/releases) page
(raw executables, no archive to extract — on Unix, `chmod +x` it and move it onto your `PATH`).

Library:

```console
go get github.com/Th0masL/contextinfo
```

## Fields

`contextinfo` reports one **flat** set of fields. Each is resolved from the best
available source — git/OS locally, with CI taking precedence where noted.

| Field | Meaning | Source |
| --- | --- | --- |
| `git_branch` | current branch (`""` on a tag/detached checkout) | git; CI hint when detached |
| `git_commit_sha` | full HEAD commit SHA | git |
| `git_commit_sha_short` | first 7 chars of `git_commit_sha` | git |
| `git_tag` | tag pointing at HEAD (`""` if none) | git |
| `git_dirty` | working tree has uncommitted changes | git |
| `git_checksum` | SHA-256 of the non-ignored working-dir files | git (`--no-checksum` to skip) |
| `git_repo_url` | HTTPS web URL of the repository | git remote (ssh→https); CI when available |
| `git_repository` | `owner/repo` slug | git remote; CI override |
| `actor` | who triggered the run | CI user, else local OS user |
| `event` | what triggered the run (`push`, `release`, …) | CI, else `manual` |
| `ci_platform` | `github-actions`, `gitlab-ci`, `circleci`, `unknown`, or `""` locally | CI |
| `ci_build_url` | current build/pipeline URL | CI |
| `ci_build_number` | build/pipeline number | CI |
| `ci_workflow` | workflow or job name | CI |
| `runtime_hostname` | `os.Hostname()` | OS |

## CLI usage

```console
$ contextinfo                      # shell NAME=value lines (default)
$ contextinfo --format=json        # flat JSON object
$ contextinfo --format=text        # aligned key/value text
$ contextinfo --format=tfvars      # Terraform variables (HCL)
$ contextinfo --format=tfvars-json # Terraform variables (JSON)
$ contextinfo --version
$ contextinfo --help               # full usage + examples
```

`envvar`, `tfvars`, and `tfvars-json` take an optional **`--prefix`** (empty by
default):

```console
$ contextinfo                                   # git_commit_sha='...', event='manual', ...
$ contextinfo --format=tfvars --prefix TF_VAR_  # TF_VAR_git_commit_sha = "..."
```

### Environment variables (default)

The default `envvar` format prints shell `NAME=value` lines (string values are
single-quoted for shell safety; booleans are bare). Combined with
`--prefix TF_VAR_`, you can export the context as Terraform input variables:

```console
$ contextinfo --format=envvar
git_branch='main'
git_commit_sha='a1b2c3d4e5f6...'
git_dirty=false
git_repository='org/repo'
actor='octocat'
event='push'
ci_platform='github-actions'

# export TF_VAR_* and run terraform with the context available:
$ set -a; eval "$(contextinfo --format=envvar --prefix TF_VAR_)"; set +a
$ terraform plan      # reads var.git_commit_sha, var.git_repository, ... from the environment
```

Flat JSON output (`--format=json`):

```json
{
  "git_branch": "main",
  "git_commit_sha": "a1b2c3d4e5f6...",
  "git_commit_sha_short": "a1b2c3d",
  "git_tag": "",
  "git_dirty": false,
  "git_checksum": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
  "git_repo_url": "https://github.com/org/repo",
  "git_repository": "org/repo",
  "actor": "octocat",
  "event": "push",
  "ci_platform": "github-actions",
  "ci_build_url": "https://github.com/org/repo/actions/runs/123",
  "ci_build_number": "7",
  "ci_workflow": "deploy",
  "runtime_hostname": "runner-xyz"
}
```

The command exits `0` even when nothing is detected (detection is never fatal).
Run locally (no CI) it reports `ci_platform=""`, `event="manual"`, and `actor`
falls back to your OS user.

### Content checksum

`git_checksum` is a SHA-256 fingerprint of the **non-ignored files in the working
directory** (`git ls-files --cached --others --exclude-standard`, sorted) — a
content identity independent of commit history. Two commits with identical files
(an empty commit, a revert) share a checksum, and uncommitted edits change it,
which the commit SHA alone can't tell you.

Symlinks are followed and the **target's** content is hashed — handy for
Terraform stacks that symlink shared files in from parent folders, so editing the
shared file moves the checksum. It is computed by default; pass `--no-checksum`
(or `contextinfo.WithoutChecksum()` in the library) to skip it when reading every
file would be too expensive on a large tree.

### Terraform variables

The `tfvars` / `tfvars-json` formats emit flat variables you can drop next to
your Terraform config — Terraform auto-loads `*.auto.tfvars` and
`*.auto.tfvars.json`:

```console
$ contextinfo --format=tfvars-json > contextinfo.auto.tfvars.json
$ contextinfo --format=tfvars      > contextinfo.auto.tfvars
```

```hcl
# contextinfo.auto.tfvars  (no prefix by default)
git_branch     = "main"
git_commit_sha = "a1b2c3d4e5f6..."
git_dirty      = false
git_repository = "org/repo"
```

Declare only the variables you use (add `--prefix` if you want them namespaced,
e.g. `--prefix tf_` → `tf_git_commit_sha`):

```hcl
variable "git_commit_sha" {
  type    = string
  default = ""
}
```

String values are safely quoted in HCL (including `${`/`%{` interpolation
markers), so untrusted ref or remote values can't break the file.

## Library usage

```go
package main

import (
	"fmt"

	"github.com/Th0masL/contextinfo/pkg/contextinfo"
)

func main() {
	info := contextinfo.Detect()
	fmt.Println(info.GitRepository, info.GitBranch, info.GitCommitSHAShort, info.Event)
}
```

`Detect()` returns a single flat [`contextinfo.Info`](pkg/contextinfo/contextinfo.go)
value (pass `contextinfo.WithoutChecksum()` to skip the file checksum). It also
offers `EnvVars(prefix)`, `FlatJSON(prefix)`, `TFVarsHCL(prefix)`, and
`TFVarsJSON(prefix)` for rendering.

## Detected CI platforms

Only platforms whose environment has been verified against real output are
recognized by name and have their CI fields populated:

| Platform | Detected via | `ci_platform` |
| --- | --- | --- |
| GitHub Actions | `GITHUB_ACTIONS=true` | `github-actions` |
| GitLab CI | `GITLAB_CI=true` | `gitlab-ci` |
| CircleCI | `CIRCLECI=true` | `circleci` |
| any other CI | `CI=true` | `unknown` |
| none (local) | — | `""` |

Other CI systems (Jenkins, Travis, Buildkite, …) are reported as `unknown` for
now — adding them requires reviewing each one's real environment variables, not
guessing. Contributions welcome.

CircleCI exposes no single "event" variable, so `event` is derived: `tag` when
`CIRCLE_TAG` is set, otherwise `push` for a branch build.

## Development

Requires Go 1.24+ and `git` on `PATH`.

```console
go build ./...
go vet ./...
go test ./... -race
go run ./cmd/contextinfo
```

The CI-detection tests run against committed environment dumps in
[`pkg/contextinfo/testdata/env`](pkg/contextinfo/testdata/env) (captured from
real GitHub Actions and GitLab CI runs), so the per-provider mapping is checked
against actual platform output rather than assumptions.

Releases are cut by tagging `vX.Y.Z`; GoReleaser builds the cross-platform
binaries and a `checksums.txt`, and attaches them to a GitHub Release (signed
when GPG secrets are configured).

## License

MIT — see [LICENSE](LICENSE).
