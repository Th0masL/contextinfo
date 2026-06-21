package contextinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo creates a temporary git repository with one committed file and an
// origin remote, and returns its path. It skips the test if git is missing.
func newRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "t@e.com")
	runGit(t, dir, "config", "user.name", "T")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	runGit(t, dir, "config", "tag.gpgsign", "false") // ignore any global tag.gpgSign
	runGit(t, dir, "remote", "add", "origin", "https://example.com/org/repo.git")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestDetectGit(t *testing.T) {
	dir := newRepo(t)
	t.Chdir(dir)
	clearCI(t)

	g := detectGit()
	if g.Commit == "" || g.Branch == "" {
		t.Fatalf("commit/branch empty: %+v", g)
	}
	if g.Dirty {
		t.Error("expected a clean tree")
	}
	if g.Remote != "https://example.com/org/repo.git" {
		t.Errorf("remote = %q", g.Remote)
	}
	if g.Tag != "" {
		t.Errorf("unexpected tag %q", g.Tag)
	}

	runGit(t, dir, "tag", "v1.0.0")
	if g := detectGit(); g.Tag != "v1.0.0" {
		t.Errorf("tag = %q, want v1.0.0", g.Tag)
	}

	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if g := detectGit(); !g.Dirty {
		t.Error("expected a dirty tree after modification")
	}
}

func TestSanitizeRemote(t *testing.T) {
	cases := map[string]string{
		// GitLab CI rewrites origin with the job token — must be stripped.
		"https://gitlab-ci-token:secrettoken@gitlab.com/o/r.git": "https://gitlab.com/o/r.git",
		"https://user:pa55w0rd@example.com/o/r.git":              "https://example.com/o/r.git",
		// No credentials / not http(s): left as-is.
		"https://github.com/o/r.git":   "https://github.com/o/r.git",
		"git@github.com:o/r.git":       "git@github.com:o/r.git",
		"ssh://git@gitlab.com/o/r.git": "ssh://git@gitlab.com/o/r.git",
		"":                             "",
	}
	for in, want := range cases {
		got := sanitizeRemote(in)
		if got != want {
			t.Errorf("sanitizeRemote(%q) = %q, want %q", in, got, want)
		}
		if strings.Contains(got, "secrettoken") || strings.Contains(got, "pa55w0rd") {
			t.Errorf("sanitizeRemote(%q) leaked a credential: %q", in, got)
		}
	}
}

func TestDetectGitOutsideRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	clearCI(t)

	g := detectGit()
	if g.Commit != "" || g.Remote != "" || g.Dirty {
		t.Errorf("expected empty git info outside a repo, got %+v", g)
	}
}
