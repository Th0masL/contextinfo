package contextinfo

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sort"
	"strings"
)

// gitChecksum returns a SHA-256 fingerprint of the non-ignored files in the
// current working directory (and below). It captures the actual content of the
// tree independently of commit history: two commits with identical files (an
// empty commit, a revert to the same state, ...) produce the same checksum, and
// uncommitted edits change it. Returns "" when not in a git repository (or when
// there are no files to hash).
//
// The file list is git's view of "not ignored" — tracked plus untracked files,
// honoring .gitignore — scoped to the working directory (git ls-files lists the
// cwd subtree). Paths come back NUL-separated (-z) so any byte in a filename is
// preserved.
//
// Symlinks are followed and the *target's* content is hashed, which matters for
// Terraform stacks that symlink shared files in from parent folders — editing
// the shared file then moves the checksum. A listed path that can't be read as a
// regular file (broken or circular symlink, directory, device, FIFO) contributes
// a deterministic placeholder instead of being read; this keeps the result
// stable and avoids blocking on special files.
func gitChecksum() string {
	out := gitOutput("ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if out == "" {
		return ""
	}
	files := strings.Split(out, "\x00")
	sort.Strings(files)

	h := sha256.New()
	for _, path := range files {
		if path == "" {
			continue
		}
		h.Write([]byte(path))
		h.Write([]byte{0}) // delimit the path from its content
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			if data, err := os.ReadFile(path); err == nil {
				h.Write(data)
				continue
			}
		}
		h.Write([]byte("<unreadable>"))
	}
	return hex.EncodeToString(h.Sum(nil))
}
