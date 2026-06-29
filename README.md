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

The core `contextinfo` package has **no external dependencies** (standard library
only) and never fails: when something can't be detected, the corresponding field
is left empty. (Only the optional `config` subpackage — for reading
`.contextinfo.yaml` — pulls in a YAML dependency.)

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
| `git_commit_subject` | HEAD commit subject (first line); user-editable, a hint | git |
| `git_is_merge` | HEAD is a merge commit (2+ parents); structural, reliable | git |
| `git_tag` | tag pointing at HEAD (`""` if none) | git |
| `git_dirty` | working tree has uncommitted changes | git |
| `files_checksum` | SHA-256 of the non-ignored working-dir files | git (`--no-files-checksum` to skip) |
| `git_repo_url` | HTTPS web URL of the repository | git remote (ssh→https); CI when available |
| `git_repository` | `owner/repo` slug | git remote; CI override |
| `actor` | who triggered the run | CI user, else local OS user |
| `event` | normalized trigger: `push`/`tag`/`pull_request`/`release`/`schedule`/`manual` | CI, else `manual` |
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
$ contextinfo --dir /path/to/repo  # inspect another directory (default: cwd)
$ contextinfo --explain            # add <name>_explained source notes
$ contextinfo --version
$ contextinfo --help               # full usage + examples
```

`envvar`, `json`, and `tfvars` take an optional **`--prefix`** (empty by
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
  "git_commit_subject": "Merge pull request #3 from org/feature",
  "git_is_merge": true,
  "git_tag": "",
  "git_dirty": false,
  "files_checksum": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
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

`files_checksum` is a SHA-256 fingerprint of the **non-ignored files in the
working directory** — a content identity independent of commit history. Two
commits with identical files (an empty commit, a revert) share a checksum, and
uncommitted edits change it, which the commit SHA alone can't tell you. Symlinks
are followed and the **target's** content is hashed (handy for Terraform stacks
that symlink shared files in from parent folders, so editing the shared file
moves the checksum).

It is **byte-for-byte reproducible** — `contextinfo` is just a native,
dependency-free implementation of this shell pipeline:

```console
$ git ls-files -z --cached --others --exclude-standard \
    | LC_ALL=C sort -z | xargs -0 -r sha256sum | sha256sum | awk '{print $1}'
```

(`LC_ALL=C` gives byte-order sorting, `-z`/`-0` handle any filename, `-r` keeps an
empty repo working.) Unreadable or non-regular paths — dangling symlinks,
directories, permission errors — are skipped, exactly as `sha256sum` skips them.

Computed by default; pass `--no-files-checksum` (or
`contextinfo.WithoutFilesChecksum()` in the library) to skip it when reading every
file would be too expensive on a large tree.

### Explaining where values came from (`--explain`)

Add `--explain` to **any** format to emit, after each field, a `<field>_explained`
companion naming the source of the value — the env var(s) or git command used. It
names variables and commands, not their contents, so it never exposes secrets.
Handy for "why is this empty / where did this come from?":

```console
$ contextinfo --explain
git_branch='main'
git_branch_explained='git symbolic-ref --short HEAD'
git_commit_sha='5d98397c…'
git_commit_sha_explained='git rev-parse HEAD'
git_tag=''
git_tag_explained='git describe --tags --exact-match (no tag at HEAD)'
event='manual'
event_explained='default (not in CI)'
…
```

In CI the notes name the winning provider variables — e.g.
`actor_explained='GITHUB_ACTOR'`, `git_repo_url_explained='GITHUB_SERVER_URL + GITHUB_REPOSITORY'`,
or `git_branch_explained='none (tag or detached HEAD)'`. It works with every
format (the companions are just extra keys), and the library does the same via
`RenderOptions{Explain: true}` (see below).

### Terraform variables

The `tfvars` (HCL) and `json` formats emit flat variables you can drop next to
your Terraform config — Terraform auto-loads `*.auto.tfvars` and
`*.auto.tfvars.json` (a flat JSON object is valid `.tfvars.json`):

```console
$ contextinfo --format=json   > contextinfo.auto.tfvars.json
$ contextinfo --format=tfvars > contextinfo.auto.tfvars
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

## Configuration file

Settings can also come from a `.contextinfo.yaml` file instead of (or alongside)
flags. contextinfo searches these locations, highest precedence first:

1. the current directory (or `--dir`) — `.contextinfo.yaml`
2. each parent directory up to the git repo root (the dir containing `.git`)
3. `$HOME/.contextinfo.yaml`
4. `/etc/contextinfo.yaml`

`.yaml` is canonical, but `.yml` is accepted as a fallback in each location
(`.yaml` wins if both exist). Files are **merged closest-wins** (a key set in the
CWD file beats the same key in a parent / `$HOME` / `/etc` file), and
**explicitly-passed flags override the file**. Keys mirror the flags:

```yaml
# .contextinfo.yaml
format: tfvars          # envvar | json | text | tfvars
prefix: TF_VAR_
files_checksum: false   # same as --no-files-checksum
explain: false
# config_cascade: false # stop the cascade here (see below)
# deploy: { ... }       # derive env_name/build_type — see "Deploy rules" below
```

A missing file is fine (no config); a malformed file is an error.

**Limiting the cascade.** Two ways to stop the merge:

- **`config_cascade: false`** in a file makes it the **top of the cascade**:
  discovery stops there, so files *farther* from the directory (parents above it,
  `$HOME`, `/etc`) are ignored, while files *closer* still merge. If several files
  set it, the one **closest** to the directory wins (farther ones are never read).
  Use it to make a repo or stack self-contained.
- **`--no-config-cascade`** (CLI; library: `config.NoCascade()`) reads **only the
  single closest** `.contextinfo.yaml` and ignores everything else — no merge at
  all. The invoker forces isolation, regardless of file contents.

Library users load the same config via the `config` subpackage — which is where
the YAML dependency lives, so the core `contextinfo` package stays
dependency-free:

```go
import (
	"github.com/Th0masL/contextinfo"
	"github.com/Th0masL/contextinfo/config"
)

cfg, _, _ := config.Load(dir) // discover + merge .contextinfo.yaml for dir
info := contextinfo.Detect(append(cfg.DetectOptions(), contextinfo.WithDir(dir))...)
out := info.FlatJSON(cfg.RenderOptions()) // cfg.RenderOptions() carries prefix + explain
// cfg.Format selects which render method to call
```

## Deploy rules

A `deploy:` block derives extra variables — typically a deployment target like
`env_name` and `build_type` — from the detected context. It's an ordered list of
rules; the **first matching rule wins**, merged over a `default`. Each rule's
`set:` is an open-ended map, so you can emit any variables you like (`cluster`,
`region`, …), not just those two.

```yaml
deploy:
  rules:                          # first match wins
    - if:
        tag:
          regex: '^v[0-9]+\.[0-9]+\.[0-9]+$'   # strict semver tag
      set: { env_name: prod, build_type: production }

    - if:
        branch: main              # bare strings are globs; `main` is exact
      set: { env_name: prod, build_type: production }

    - if:
        branch: "release/*"
      set: { env_name: dev, build_type: staging }   # no staging env → dev

    - if:                         # (develop OR feature/*) AND not a PR build
        all:
          - any:
              - { branch: develop }
              - { branch: "feature/*" }
          - not: { event: pull_request }
      set: { env_name: dev, build_type: development }

  default:
    set: { env_name: dev, build_type: development }
```

**Conditions** (`if:`) are a small boolean tree, so you get full `&&`/`||`/`()`
without an expression language:

- A plain mapping is **AND** across fields: `{ branch: main, event: push }`.
- `all:` / `any:` / `not:` are **AND** / **OR** / **NOT**; nest them to group.
- A field value can be one pattern or a **list** (OR over values):
  `event: [push, manual]`.

**Matching** a field value:

- A bare string is a **glob** — `*` matches any run of characters (including
  `/`), `?` matches one, everything else is literal and anchored (so `main`
  matches only `main`, `release/*` matches `release/anything`).
- `{ regex: '…' }` is a full Go regexp (anchor it yourself; single-quote it so
  backslashes stay literal) — use it for strict patterns like semver.
- `{ glob: '…' }` is an explicit glob, if you ever want it spelled out.

**Matchable fields:** any output field, by its output name — `git_branch`,
`git_commit_sha`, `git_commit_sha_short`, `git_commit_subject`, `git_is_merge`,
`git_tag`, `git_dirty`, `files_checksum`, `git_repo_url`, `git_repository`,
`actor`, `event`, `ci_platform`, `ci_build_url`, `ci_build_number`,
`ci_workflow`, `runtime_hostname`. The `git_*` fields also accept a short alias
(`branch`, `tag`, `commit_sha`, `commit_sha_short`, `commit_subject`, `is_merge`,
`dirty`, `repo_url`, `repository`), so `branch: main` and `git_branch: main` are
equivalent. An unknown field name is a load error, so typos surface immediately.

**Detecting a merge.** `git_is_merge` is the *structural* signal (HEAD has 2+
parents) and `git_commit_subject` is a *heuristic* one (the merge message, which
is user-editable). `git_is_merge` is only meaningful on a `push` — on a
`pull_request` build it's provider-dependent (GitHub checks out a synthetic
2-parent merge ref, GitLab the source branch). So gate on the event:

```yaml
- if:
    all:
      - branch: main
      - event: push
      - any:
          - { git_is_merge: "true" }                                   # merge commit landed
          - { git_commit_subject: { regex: '^Merge (pull request|branch) ' } }
          - { git_commit_subject: { regex: '\(#[0-9]+\)$' } }          # squash "(#123)"
  set: { env_name: prod, build_type: production }
```

The resolved variables appear as additional output fields in every format, after
the detected ones; `--explain` notes which rule set each (`deploy: rule #2
matched`, `deploy: default`, or `deploy: explicit override`). To force a value
from the command line, overriding the rules:

```sh
contextinfo --env-name=prod --build-type=production
```

> contextinfo is local-first, so a local checkout reports `event=manual` (not
> `push`). If a rule should fire both in CI and locally on a branch, match on
> `branch` alone rather than gating on `event: push`.

## Library usage

```go
package main

import (
	"fmt"

	"github.com/Th0masL/contextinfo"
)

func main() {
	info := contextinfo.Detect()
	fmt.Println(info.GitRepository, info.GitBranch, info.GitCommitSHAShort, info.Event)
}
```

`Detect()` returns a single flat [`contextinfo.Info`](contextinfo.go) value.
Detection options: `contextinfo.WithDir(path)` inspects another directory and
`contextinfo.WithoutFilesChecksum()` skips the file checksum. `Detect` holds no
global state, so it is safe to call concurrently for different directories (e.g.
one goroutine per Terraform stack). For long-running embedders,
`contextinfo.DetectContext(ctx, opts...)` bounds the git subprocesses with a
context (cancel/timeout). Deploy rules can be loaded from a `.contextinfo.yaml`
(via the `config` subpackage) **or built in code** with the
[`deploy`](deploy) package — handy for an embedder such as a Terraform provider
that decodes rules from HCL. Apply them with `contextinfo.WithDeployRules(...)`
or `contextinfo.Resolve(rules, info)`.

Rendering is separate from detection: `EnvVars`, `FlatJSON`, `TFVarsHCL`, and
`Text` each take a `contextinfo.RenderOptions{Prefix, Explain}` — so the same
`Info` can be rendered with or without a prefix and with or without the
`<field>_explained` companions.

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

`event` is **normalized** to a common vocabulary across providers — `push`,
`tag`, `pull_request`, `release`, `schedule`, `manual` — so a GitLab tag pipeline
and a GitHub tag push both report `tag` (uncommon platform triggers pass through
their raw value). CircleCI has no event variable, so it's derived from
`CIRCLE_TAG` / `CIRCLE_PULL_REQUEST` / `CIRCLE_BRANCH`.

### Detection reference (verify it yourself)

contextinfo's fields are the **normalized conclusions** of analyzing **raw
inputs** — provider environment variables and local `git`. These tables show that
mapping (raw input → normalized output) so you can reproduce every value by hand.
The shown values were captured from real runs (see the `test-printenv` sandbox).

The headline normalization is **`event`**: it is *not* a raw variable but a
conclusion — contextinfo collapses each provider's trigger into one fixed
vocabulary (`push`, `tag`, `pull_request`, `release`, `schedule`, `manual`) so the
value means the same thing everywhere, even though each provider signals it
differently:

| normalized `event` (output) | GitHub `GITHUB_EVENT_NAME` (+`GITHUB_REF_TYPE`) | GitLab `CI_PIPELINE_SOURCE` (+`CI_COMMIT_TAG`) | CircleCI (ref variable) |
| --- | --- | --- | --- |
| `push` | `push` + `branch` | `push`, no `CI_COMMIT_TAG` | `CIRCLE_BRANCH` set |
| `tag` | `push` + `tag` | `push` + `CI_COMMIT_TAG` | `CIRCLE_TAG` set |
| `pull_request` | `pull_request` / `pull_request_target` | `merge_request_event` / `external_pull_request_event` | `CIRCLE_PULL_REQUEST` set |
| `release` | `release` | *(none — no native event)* | *(none)* |
| `schedule` | `schedule` | `schedule` | *(none)* |
| `manual` | `workflow_dispatch` / `repository_dispatch` | `web` | *(none)* |
| *(anything else)* | passed through unchanged | passed through unchanged | `""` |

> **`release` vs `tag`, and detecting a tagged build.** Only GitHub has a distinct
> `release` event, because the platforms model releases oppositely: a **GitHub**
> Release *creates the tag and fires `release`* (so you see both `event=release`
> and, from the tag-ref push, `event=tag`). On **GitLab** the **tag is created
> first** (→ `event=tag`), and publishing a Release afterward **triggers no
> pipeline at all** — it's invisible to CI; the tag pipeline is the only thing that
> runs. (CircleCI likewise only has `tag`.) A GitHub release build is
> `event=release` (not `tag`) but **still sets `git_tag`**. So to detect "any
> tagged/version build" portably, check **`git_tag != ""`** (set for both `tag`
> and `release`) rather than `event=tag` alone.

**Git commands — run locally in every provider** (via `git -C <dir>`; the
sha/parents/subject come from one combined `git log -1 --format='%H%x00%P%x00%s'`):

| field | obtained from | notes |
| --- | --- | --- |
| *(gate)* | `git rev-parse --is-inside-work-tree` | if not `true`, all git fields below are skipped |
| `git_commit_sha` | `git log -1 --format=%H` | |
| `git_commit_sha_short` | first 7 chars of the SHA | |
| `git_commit_subject` | `git log -1 --format=%s` | first line; **user-editable** (a hint) |
| `git_is_merge` | `git show -s --format=%P` → 2+ parents | structural merge signal (reliable) |
| `git_tag` | `git describe --tags --exact-match` | empty when HEAD isn't tagged |
| `git_dirty` | `git status --porcelain` non-empty | |
| `git_branch` | `git symbolic-ref --short HEAD`, else the CI branch hint | hint used only when HEAD is detached |
| `git_repository` / `git_repo_url` | `git config --get remote.origin.url` (ssh→https, credentials stripped) | the CI value wins when set |
| `files_checksum` | SHA-256 over `git ls-files` (see [Content checksum](#content-checksum)) | |

The per-provider tables below read **left → right: raw env-var inputs, then the
normalized fields contextinfo outputs** for that scenario (the `→` columns).

**GitHub Actions:**

| Scenario | Raw env-var inputs | → `event` | → `git_branch` (source) | → `git_is_merge` |
| --- | --- | --- | --- | --- |
| push to a branch | `GITHUB_EVENT_NAME=push`, `GITHUB_REF_TYPE=branch` | `push` | branch (`git symbolic-ref`; attached) | `false` |
| push a tag | `GITHUB_EVENT_NAME=push`, `GITHUB_REF_TYPE=tag` | `tag` | `""` | `false` |
| open / update a PR | `GITHUB_EVENT_NAME=pull_request`, `GITHUB_HEAD_REF=<src>`, `GITHUB_REF=refs/pull/N/merge` | `pull_request` | `<src>` (`GITHUB_HEAD_REF`; HEAD detached) | **`true`** ⚠ synthetic merge ref — *not* a merge to main |
| PR merged → main | `GITHUB_EVENT_NAME=push`, `GITHUB_REF_NAME=main` | `push` | `main` | `true` (+ subject `Merge pull request #N …`) |
| publish a release | `GITHUB_EVENT_NAME=release` | `release` | `""` | — |
| scheduled run | `GITHUB_EVENT_NAME=schedule` | `schedule` | branch (`GITHUB_REF_NAME`) | — |
| manual run | `GITHUB_EVENT_NAME=workflow_dispatch` | `manual` | branch (`GITHUB_REF_NAME`) | — |

Branch hint precedence: `GITHUB_HEAD_REF` → `GITHUB_REF_NAME` (only when
`GITHUB_REF_TYPE=branch`). Any other `GITHUB_EVENT_NAME` passes through verbatim.

**GitLab CI:**

| Scenario | Raw env-var inputs | → `event` | → `git_branch` (source) | → `git_is_merge` |
| --- | --- | --- | --- | --- |
| push to a branch | `CI_PIPELINE_SOURCE=push`, `CI_COMMIT_BRANCH=<branch>`, `CI_COMMIT_TAG=` | `push` | `<branch>` (`CI_COMMIT_BRANCH`; HEAD detached) | `false` |
| push a tag | `CI_PIPELINE_SOURCE=push`, `CI_COMMIT_TAG=<tag>` | `tag` | `""` | `false` |
| open / update an MR | `CI_PIPELINE_SOURCE=merge_request_event`, `CI_MERGE_REQUEST_SOURCE_BRANCH_NAME=<src>`, `CI_COMMIT_BRANCH=` | `pull_request` | `<src>` (MR source) | `false` (runs on source HEAD) |
| MR merged → main | `CI_PIPELINE_SOURCE=push`, `CI_COMMIT_BRANCH=main` | `push` | `main` | `true` (+ subject `Merge branch '…' into 'main'`) |
| scheduled run | `CI_PIPELINE_SOURCE=schedule` | `schedule` | `CI_COMMIT_BRANCH` | — |
| manual ("Run pipeline") | `CI_PIPELINE_SOURCE=web` | `manual` | `CI_COMMIT_BRANCH` | — |

Branch hint precedence: `CI_COMMIT_BRANCH` → `CI_MERGE_REQUEST_SOURCE_BRANCH_NAME`
(GitLab's checkout is always detached, so the hint is always used).

**CircleCI** — no event variable; `event` is derived from the ref (first match wins):

| Scenario | Raw env-var inputs | → `event` | → `git_branch` (source) |
| --- | --- | --- | --- |
| tag build | `CIRCLE_TAG=<tag>` | `tag` | `""` |
| PR build | `CIRCLE_PULL_REQUEST=<url>` | `pull_request` | `CIRCLE_BRANCH` |
| branch build | `CIRCLE_BRANCH=<branch>` | `push` | `<branch>` (`CIRCLE_BRANCH`) |
| none of the above | — | `""` | — |

`git_is_merge` follows the git rule above; `git_repo_url` falls back to the local
git remote when `CIRCLE_REPOSITORY_URL` is empty.

> **Merge caveat.** `git_is_merge` is reliable on `event=push` (a real merge
> commit has 2 parents), but **provider-dependent on PR/MR builds**: GitHub checks
> out a synthetic 2-parent merge ref (`true`), GitLab runs on the source branch
> (`false`). So gate merge rules on `event=push`.

## Development

Requires Go 1.21+ and `git` on `PATH`.

```console
go build ./...
go vet ./...
go test ./... -race
go run ./cmd/contextinfo
```

The CI-detection tests run against committed environment dumps in
[`internal/ci/testdata/env`](internal/ci/testdata/env) (captured from
real GitHub Actions and GitLab CI runs), so the per-provider mapping is checked
against actual platform output rather than assumptions.

Releases are cut by tagging `vX.Y.Z`; GoReleaser builds the cross-platform
binaries and a `checksums.txt`, and attaches them to a GitHub Release (signed
when GPG secrets are configured).

## License

MIT — see [LICENSE](LICENSE).
