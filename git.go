package contextinfo

import (
	"context"
	"os/exec"
	"strings"
)

// gitOutput runs a git subcommand in dir (or the process working directory when
// dir is "") and returns its trimmed stdout, or "" on any error (no git binary,
// not a repo, no commits, detached HEAD, a cancelled ctx, ...). Detection never
// fails — empty means "unknown". ctx bounds the subprocess so an embedding
// service can cancel or time out a hung git.
func gitOutput(ctx context.Context, dir string, args ...string) string {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir // "" means the process's current directory
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitBranch returns the current branch of the repo in dir and a note of where it
// came from. On a branch, symbolic-ref is authoritative; in CI's detached-HEAD
// checkout it falls back to the CI branch hint (never a tag name — see the
// per-provider detectors), labelled with hintSource.
func gitBranch(ctx context.Context, dir, hint, hintSource string) (value, source string) {
	if b := gitOutput(ctx, dir, "symbolic-ref", "--short", "HEAD"); b != "" {
		return b, "git symbolic-ref --short HEAD"
	}
	if hint != "" {
		return hint, hintSource
	}
	return "", "none (detached HEAD or not a git repository)"
}

// gitDirty reports whether the working tree in dir has uncommitted changes.
func gitDirty(ctx context.Context, dir string) bool {
	return gitOutput(ctx, dir, "status", "--porcelain") != ""
}

// gitRemoteURL returns the origin remote URL of the repo in dir with any embedded
// credentials stripped, or "" when there is no origin.
func gitRemoteURL(ctx context.Context, dir string) string {
	return sanitizeRemote(gitOutput(ctx, dir, "config", "--get", "remote.origin.url"))
}
