// Package git wraps the git subprocesses contextinfo needs. Every call is bounded
// by a context and returns empty/false on any error (no git binary, not a repo,
// detached HEAD, cancelled ctx, ...), so detection never fails — empty means
// "unknown".
package git

import (
	"context"
	"os/exec"
	"strings"

	"github.com/Th0masL/contextinfo/internal/scm"
)

// Output runs a git subcommand in dir (or the process working directory when dir
// is "") and returns its trimmed stdout, or "" on any error. ctx bounds the
// subprocess so a long-running embedder can cancel or time out a hung git.
func Output(ctx context.Context, dir string, args ...string) string {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir // "" means the process's current directory
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// InRepo reports whether dir is inside a git work tree. The caller uses this to
// skip the other git calls (and the file checksum) when there is no repository.
func InRepo(ctx context.Context, dir string) bool {
	return Output(ctx, dir, "rev-parse", "--is-inside-work-tree") == "true"
}

// Commit returns HEAD's full SHA, its parent count, and its subject line — in a
// single call (NUL-separated so the subject can't be confused with the rest). A
// parent count >= 2 means HEAD is a merge commit. All-zero/empty when HEAD is
// unborn (empty repo) or not a repository.
func Commit(ctx context.Context, dir string) (sha string, parents int, subject string) {
	out := Output(ctx, dir, "log", "-1", "--format=%H%x00%P%x00%s")
	f := strings.SplitN(out, "\x00", 3)
	if len(f) < 3 {
		return "", 0, ""
	}
	return f[0], len(strings.Fields(f[1])), f[2]
}

// Branch returns the current branch of the repo in dir and a note of where it
// came from. On a branch, symbolic-ref is authoritative; in CI's detached-HEAD
// checkout it falls back to the CI branch hint (never a tag name — see the
// per-provider detectors), labelled with hintSource.
func Branch(ctx context.Context, dir, hint, hintSource string) (value, source string) {
	if b := Output(ctx, dir, "symbolic-ref", "--short", "HEAD"); b != "" {
		return b, "git symbolic-ref --short HEAD"
	}
	if hint != "" {
		return hint, hintSource
	}
	return "", "none (detached HEAD or not a git repository)"
}

// Dirty reports whether the working tree in dir has uncommitted changes.
func Dirty(ctx context.Context, dir string) bool {
	return Output(ctx, dir, "status", "--porcelain") != ""
}

// RemoteURL returns the origin remote URL of the repo in dir with any embedded
// credentials stripped, or "" when there is no origin.
func RemoteURL(ctx context.Context, dir string) string {
	return scm.Sanitize(Output(ctx, dir, "config", "--get", "remote.origin.url"))
}
