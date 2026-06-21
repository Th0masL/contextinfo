package contextinfo

import (
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
		Remote: remote,
	}
}

// gitBranch returns the current branch, falling back to well-known CI
// environment variables when HEAD is detached (common in CI checkouts).
func gitBranch() string {
	if b, err := git("symbolic-ref", "--short", "HEAD"); err == nil && b != "" {
		return b
	}
	for _, k := range []string{
		"GITHUB_HEAD_REF", "GITHUB_REF_NAME",
		"CI_COMMIT_REF_NAME", "CIRCLE_BRANCH",
		"BUILDKITE_BRANCH", "TRAVIS_BRANCH", "BRANCH_NAME",
	} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
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
