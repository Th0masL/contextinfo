package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The checksum is deterministic for an unchanged tree and moves whenever a
// tracked file is edited or a new non-ignored file appears.
func TestChecksumStableAndSensitive(t *testing.T) {
	dir := newRepo(t)
	ctx := context.Background()

	sum1 := Checksum(ctx, dir)
	if sum1 == "" {
		t.Fatal("empty checksum in a repo")
	}
	if Checksum(ctx, dir) != sum1 {
		t.Error("checksum is not stable across runs with no change")
	}

	// Editing a tracked file changes the checksum.
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum2 := Checksum(ctx, dir)
	if sum2 == sum1 {
		t.Error("checksum unchanged after editing a tracked file")
	}

	// A new untracked (non-ignored) file changes the checksum.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if Checksum(ctx, dir) == sum2 {
		t.Error("checksum unchanged after adding an untracked file")
	}
}

// Files excluded by .gitignore must not affect the checksum.
func TestChecksumIgnoresGitignored(t *testing.T) {
	dir := newRepo(t)
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := Checksum(ctx, dir) // .gitignore itself is a non-ignored file -> counted once

	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if Checksum(ctx, dir) != base {
		t.Error("checksum changed after adding a git-ignored file")
	}
}

// A symlink contributes its target's content, so editing the linked file moves
// the checksum — the case Terraform stacks rely on when symlinking shared files.
func TestChecksumFollowsSymlinkTarget(t *testing.T) {
	dir := newRepo(t)
	ctx := context.Background()
	shared := filepath.Join(t.TempDir(), "shared.tf")
	if err := os.WriteFile(shared, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shared, filepath.Join(dir, "shared.tf")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	runGit(t, dir, "add", "shared.tf")

	sum1 := Checksum(ctx, dir)
	if sum1 == "" {
		t.Fatal("empty checksum")
	}
	if err := os.WriteFile(shared, []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if Checksum(ctx, dir) == sum1 {
		t.Error("checksum did not change when the symlink target content changed")
	}
}

// A dangling symlink must be tolerated (skipped, like sha256sum), not error.
func TestChecksumBrokenSymlinkSafe(t *testing.T) {
	dir := newRepo(t)
	if err := os.Symlink(filepath.Join(dir, "does-not-exist"), filepath.Join(dir, "dangling")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	runGit(t, dir, "add", "dangling")
	if Checksum(context.Background(), dir) == "" {
		t.Error("a broken symlink should not blank the checksum")
	}
}

// Outside a git repository the checksum is empty (it has no file list to hash).
func TestChecksumOutsideRepo(t *testing.T) {
	if s := Checksum(context.Background(), t.TempDir()); s != "" {
		t.Errorf("checksum outside a repo = %q, want empty", s)
	}
}

// Checksum(dir) must equal the documented shell pipeline run in dir, byte-for-byte.
// This is the contract: anyone can reproduce files_checksum with the one-liner.
// Skipped where the GNU tools it uses aren't available.
func TestChecksumMatchesShellPipeline(t *testing.T) {
	for _, bin := range []string{"bash", "sha256sum"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not available", bin)
		}
	}
	dir := newRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "z.txt"), []byte("zzz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "shared.tf")
	if err := os.WriteFile(target, []byte("shared\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link.tf")); err == nil {
		runGit(t, dir, "add", "link.tf")
	}
	// Names that exercise the coreutils escaping branch (a backslash and a newline,
	// which sha256sum prefixes with "\" and escapes as \\ and \n) plus a non-ASCII
	// name (high bytes passed through verbatim, and byte-sorted the same as
	// LC_ALL=C). These are exactly the paths the ASCII-only cases above never reach.
	// (\r is intentionally omitted: not every coreutils version escapes it, which
	// would make this cross-implementation oracle non-portable.)
	for _, name := range []string{"café.txt", `back\slash.txt`, "line\nbreak.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o644); err != nil {
			t.Skipf("filesystem rejects %q: %v", name, err)
		}
	}

	const pipeline = `git ls-files -z --cached --others --exclude-standard | ` +
		`LC_ALL=C sort -z | xargs -0 -r sha256sum | sha256sum | awk '{print $1}'`
	cmd := exec.Command("bash", "-c", pipeline)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("shell pipeline failed (missing tool such as sort -z / xargs -0?): %v", err)
	}
	want := strings.TrimSpace(string(out))
	if got := Checksum(context.Background(), dir); got != want {
		t.Errorf("Checksum(dir) = %q\nshell pipeline = %q", got, want)
	}
}
