package contextinfo

import (
	"os/exec"
	"strings"
)

// gitOutput runs a git subcommand in the current working directory and returns
// its trimmed stdout, or "" on any error (no git binary, not a repo, no commits,
// detached HEAD, ...). Detection never fails — empty means "unknown".
func gitOutput(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitBranch returns the current branch. On a branch, symbolic-ref is
// authoritative; in CI's detached-HEAD checkout it falls back to the CI branch
// hint, which is never a tag name (see the per-provider detectors).
func gitBranch(hint string) string {
	if b := gitOutput("symbolic-ref", "--short", "HEAD"); b != "" {
		return b
	}
	return hint
}

// gitDirty reports whether the working tree has uncommitted changes.
func gitDirty() bool {
	return gitOutput("status", "--porcelain") != ""
}

// gitRemoteURL returns the origin remote URL with any embedded credentials
// stripped, or "" when there is no origin.
func gitRemoteURL() string {
	return sanitizeRemote(gitOutput("config", "--get", "remote.origin.url"))
}
