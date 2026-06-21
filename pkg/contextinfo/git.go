package contextinfo

import (
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// git runs a git subcommand in the current working directory and returns the
// trimmed standard output.
func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// detectGit gathers git state from the current working directory. Missing values
// (no git binary, not a repo, no commits, ...) are returned empty.
func detectGit() GitInfo {
	commit, _ := git("rev-parse", "HEAD")
	tag, _ := git("describe", "--tags", "--exact-match")
	remote, _ := git("config", "--get", "remote.origin.url")
	return GitInfo{
		Commit: commit,
		Branch: gitBranch(),
		Tag:    tag,
		Dirty:  gitDirty(),
		Remote: sanitizeRemote(remote),
	}
}

// sanitizeRemote strips embedded credentials from an http(s) remote URL. CI
// checkouts often rewrite origin to include a token (e.g. GitLab's
// "https://gitlab-ci-token:<token>@gitlab.com/..."), which must never be
// reported — it would leak into output, tfvars, or Terraform state. SSH and
// scp-style remotes (git@host:path) carry no secret and are left untouched.
func sanitizeRemote(raw string) string {
	if !strings.Contains(raw, "://") {
		return raw // scp-like (git@host:path): the user is an SSH login, not a secret
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	switch u.Scheme {
	case "http", "https":
		u.User = nil // tokens/passwords live in the userinfo of CI checkout URLs
		return u.String()
	default:
		return raw // ssh://, git://: no password in the URL
	}
}

// gitBranch returns the current branch. On a branch, symbolic-ref is
// authoritative. In CI the checkout is usually a detached HEAD, so it falls back
// to CI hints — chosen so a *tag* checkout is never mislabeled as a branch (on a
// tag event GITHUB_REF_NAME / CI_COMMIT_REF_NAME hold the tag name, not a branch).
func gitBranch() string {
	if b, err := git("symbolic-ref", "--short", "HEAD"); err == nil && b != "" {
		return b
	}
	// GITHUB_HEAD_REF is a PR source branch; GITHUB_REF_NAME is a branch only
	// when the ref type is "branch" (it's the tag name on tag/release events).
	if v := strings.TrimSpace(os.Getenv("GITHUB_HEAD_REF")); v != "" {
		return v
	}
	if os.Getenv("GITHUB_REF_TYPE") == "branch" {
		if v := strings.TrimSpace(os.Getenv("GITHUB_REF_NAME")); v != "" {
			return v
		}
	}
	// GitLab: CI_COMMIT_BRANCH is set only on branch pipelines (empty on tags).
	if v := strings.TrimSpace(os.Getenv("CI_COMMIT_BRANCH")); v != "" {
		return v
	}
	return ""
}

// gitDirty reports whether the working tree has uncommitted changes.
func gitDirty() bool {
	out, err := git("status", "--porcelain")
	if err != nil {
		return false
	}
	return out != ""
}
