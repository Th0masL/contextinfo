package git

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Checksum returns a SHA-256 fingerprint of the non-ignored files in dir (and
// below). It is a native, byte-for-byte reimplementation of this shell pipeline:
//
//	git ls-files -z --cached --others --exclude-standard \
//	    | LC_ALL=C sort -z | xargs -0 -r sha256sum | sha256sum | awk '{print $1}'
//
// That is: take git's "not ignored" file list (tracked + untracked, honoring
// .gitignore), byte-sort it, render one coreutils sha256sum line per file
// ("<hex>  <path>\n", symlinks followed and the target's content hashed), and
// return the SHA-256 of the concatenated lines. Unreadable or non-regular paths
// (dangling or circular symlinks, directories, permission errors) are skipped —
// exactly as sha256sum skips them.
//
// The value is a content fingerprint independent of commit history: two commits
// with identical files (an empty commit, a revert) share a checksum, and
// uncommitted edits change it. An empty-but-real repository yields the SHA-256 of
// an empty manifest (e3b0c442...), matching the pipeline. Returns "" only when
// not in a git repository (or git is unavailable).
func Checksum(ctx context.Context, dir string) string {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	cmd.Dir = dir // "" means the process's current directory
	out, err := cmd.Output()
	if err != nil {
		return "" // not a git repository (or git unavailable)
	}
	files := splitNUL(out)
	sort.Strings(files)

	h := sha256.New()
	for _, path := range files {
		if ctx.Err() != nil {
			return "" // cancelled mid-hash on a large tree
		}
		if line, ok := sha256sumLine(dir, path); ok {
			io.WriteString(h, line)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// splitNUL splits NUL-separated git output into entries, dropping the trailing
// empty element. Returns nil for empty input.
func splitNUL(out []byte) []string {
	s := strings.TrimRight(string(out), "\x00")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\x00")
}

// sha256sumLine renders the GNU coreutils sha256sum output line for path (which
// is relative to dir, as git ls-files reports it): "<hex>  <path>\n", or
// "\<hex>  <escaped>\n" when the path contains a backslash, newline, or carriage
// return (coreutils prefixes the line with "\" and escapes those bytes). The
// content is hashed with symlinks followed. ok is false when the path can't be
// read as a regular file, which sha256sum reports as an error and omits from its
// output.
func sha256sumLine(dir, path string) (string, bool) {
	full := path
	if dir != "" {
		full = filepath.Join(dir, path)
	}
	info, err := os.Stat(full) // Stat follows symlinks, like sha256sum
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	f, err := os.Open(full)
	if err != nil {
		return "", false
	}
	defer f.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, f); err != nil {
		return "", false
	}
	hexsum := hex.EncodeToString(digest.Sum(nil))

	if strings.ContainsAny(path, "\\\n\r") {
		esc := strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`).Replace(path)
		return "\\" + hexsum + "  " + esc + "\n", true
	}
	return hexsum + "  " + path + "\n", true
}
