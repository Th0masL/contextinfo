# contextinfo

[![CI](https://github.com/Th0masL/contextinfo/actions/workflows/ci.yml/badge.svg)](https://github.com/Th0masL/contextinfo/actions/workflows/ci.yml)

Detect the execution context of a process — the **CI/CD platform**, **git state**,
and **host runtime** — as a clean Go library and a standalone binary.

`contextinfo` has **no external dependencies** (standard library only) and never
fails: when something can't be detected (no git, not in CI), the corresponding
fields are simply left empty.

It also backs [`terraform-provider-contextinfo`](https://github.com/Th0masL/terraform-provider-contextinfo),
which exposes the same detection as a Terraform data source.

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

## CLI usage

```console
$ contextinfo                      # nested JSON (default)
$ contextinfo --format=json-flat   # flat JSON (ci.name -> ci_name)
$ contextinfo --format=text        # aligned key/value text
$ contextinfo --format=tfvars      # Terraform variables (HCL)
$ contextinfo --format=tfvars-json # Terraform variables (JSON)
$ contextinfo --version
```

The flat formats (`json-flat`, `tfvars`, `tfvars-json`) join nested paths with
`_` and take an optional **`--prefix`**, which is empty by default:

```console
$ contextinfo --format=json-flat                       # git_commit, runtime_os, ...
$ contextinfo --format=tfvars --prefix TF_VAR_         # TF_VAR_git_commit = "..."
```

Example JSON output:

```json
{
  "ci": {
    "detected": true,
    "name": "github-actions",
    "build_url": "https://github.com/org/repo/actions/runs/123",
    "build_number": "7"
  },
  "git": {
    "commit": "a1b2c3d4...",
    "branch": "main",
    "tag": "",
    "dirty": false,
    "remote": "git@github.com:org/repo.git"
  },
  "runtime": {
    "os": "linux",
    "arch": "amd64",
    "hostname": "runner-xyz"
  }
}
```

The command exits `0` even when nothing is detected (detection is never fatal).

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
ci_name    = "github-actions"
git_commit = "a1b2c3d4"
git_dirty  = false
runtime_os = "linux"
```

Declare only the variables you use (add `--prefix` if you want them namespaced,
e.g. `--prefix tf_` → `tf_git_commit`):

```hcl
variable "git_commit" {
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
	fmt.Println(info.CI.Name, info.Git.Commit, info.Runtime.OS)
}
```

`Detect()` returns a single [`contextinfo.Info`](pkg/contextinfo/contextinfo.go)
value with `CI`, `Git`, and `Runtime` sub-structs.

## Detected CI platforms

| Platform | Detected via | `name` |
| --- | --- | --- |
| GitHub Actions | `GITHUB_ACTIONS=true` | `github-actions` |
| GitLab CI | `GITLAB_CI=true` | `gitlab-ci` |
| CircleCI | `CIRCLECI=true` | `circleci` |
| Jenkins | `JENKINS_URL` set | `jenkins` |
| Travis CI | `TRAVIS=true` | `travis-ci` |
| Buildkite | `BUILDKITE=true` | `buildkite` |
| (other) | `CI=true` | `unknown` |
| none | — | `local` (`detected=false`) |

`build_url` and `build_number` are populated per platform where available.

## Development

Requires Go 1.24+ and `git` on `PATH`.

```console
go build ./...
go vet ./...
go test ./... -race
go run ./cmd/contextinfo
```

Releases are cut by tagging `vX.Y.Z`; GoReleaser builds the cross-platform
binaries and a `checksums.txt`, and attaches them to a GitHub Release (signed
when GPG secrets are configured).

## License

MIT — see [LICENSE](LICENSE).
