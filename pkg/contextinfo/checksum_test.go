package contextinfo

import (
	"os"
	"path/filepath"
	"testing"
)

// The checksum is deterministic for an unchanged tree and moves whenever a
// tracked file is edited or a new non-ignored file appears.
func TestGitChecksumStableAndSensitive(t *testing.T) {
	dir := newRepo(t)
	t.Chdir(dir)

	sum1 := gitChecksum()
	if sum1 == "" {
		t.Fatal("empty checksum in a repo")
	}
	if gitChecksum() != sum1 {
		t.Error("checksum is not stable across runs with no change")
	}

	// Editing a tracked file changes the checksum.
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum2 := gitChecksum()
	if sum2 == sum1 {
		t.Error("checksum unchanged after editing a tracked file")
	}

	// A new untracked (non-ignored) file changes the checksum.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if gitChecksum() == sum2 {
		t.Error("checksum unchanged after adding an untracked file")
	}
}

// Files excluded by .gitignore must not affect the checksum.
func TestGitChecksumIgnoresGitignored(t *testing.T) {
	dir := newRepo(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := gitChecksum() // .gitignore itself is a non-ignored file -> counted once

	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if gitChecksum() != base {
		t.Error("checksum changed after adding a git-ignored file")
	}
}

// A symlink contributes its target's content, so editing the linked file moves
// the checksum — the case Terraform stacks rely on when symlinking shared files.
func TestGitChecksumFollowsSymlinkTarget(t *testing.T) {
	dir := newRepo(t)
	// A shared file outside the repo working tree (e.g. a Terraform module
	// symlinked into a stack from a parent folder).
	shared := filepath.Join(t.TempDir(), "shared.tf")
	if err := os.WriteFile(shared, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shared, filepath.Join(dir, "shared.tf")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	runGit(t, dir, "add", "shared.tf")
	t.Chdir(dir)

	sum1 := gitChecksum()
	if sum1 == "" {
		t.Fatal("empty checksum")
	}
	// Changing the TARGET's content must move the checksum (the link is followed).
	if err := os.WriteFile(shared, []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if gitChecksum() == sum1 {
		t.Error("checksum did not change when the symlink target content changed")
	}
}

// A dangling symlink must be tolerated (placeholder), not error or blank the sum.
func TestGitChecksumBrokenSymlinkSafe(t *testing.T) {
	dir := newRepo(t)
	if err := os.Symlink(filepath.Join(dir, "does-not-exist"), filepath.Join(dir, "dangling")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	runGit(t, dir, "add", "dangling")
	t.Chdir(dir)
	if gitChecksum() == "" {
		t.Error("a broken symlink should not blank the checksum")
	}
}

// Outside a git repository the checksum is empty (it has no file list to hash).
func TestGitChecksumOutsideRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	if s := gitChecksum(); s != "" {
		t.Errorf("checksum outside a repo = %q, want empty", s)
	}
}
