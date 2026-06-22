package contextinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo creates a temporary git repository on branch "main" with one committed
// file and an scp-style origin remote, and returns its path. It skips the test
// if git is missing.
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
	runGit(t, dir, "config", "tag.gpgsign", "false")
	runGit(t, dir, "remote", "add", "origin", "git@github.com:acme/widgets.git")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "init")
	runGit(t, dir, "branch", "-M", "main") // deterministic branch name across git versions
	return dir
}

// runGit runs a git command in dir and fails the test on a non-zero exit.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// On a real repo the git helpers report the commit, branch, clean/dirty state,
// origin remote, and tag.
func TestGitOnRepo(t *testing.T) {
	dir := newRepo(t)
	t.Chdir(dir)

	if sha := gitOutput("rev-parse", "HEAD"); len(sha) != 40 {
		t.Errorf("sha = %q (len %d), want 40 hex chars", sha, len(sha))
	}
	if b := gitBranch(""); b != "main" {
		t.Errorf("branch = %q, want main", b)
	}
	if gitDirty() {
		t.Error("expected a clean tree")
	}
	if got := gitRemoteURL(); got != "git@github.com:acme/widgets.git" {
		t.Errorf("remote = %q", got)
	}
	if got := gitOutput("describe", "--tags", "--exact-match"); got != "" {
		t.Errorf("unexpected tag %q", got)
	}

	runGit(t, dir, "tag", "v1.0.0")
	if got := gitOutput("describe", "--tags", "--exact-match"); got != "v1.0.0" {
		t.Errorf("tag = %q, want v1.0.0", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !gitDirty() {
		t.Error("expected a dirty tree after modification")
	}
}

// On a detached HEAD (as in CI) symbolic-ref fails, so gitBranch returns the
// supplied hint, or "" when there is none.
func TestGitBranchDetachedUsesHint(t *testing.T) {
	dir := newRepo(t)
	t.Chdir(dir)
	runGit(t, dir, "checkout", "--detach") // mimic a CI checkout

	if b := gitBranch("feature/x"); b != "feature/x" {
		t.Errorf("detached: branch = %q, want hint feature/x", b)
	}
	if b := gitBranch(""); b != "" {
		t.Errorf("detached without hint: branch = %q, want empty", b)
	}
}

// Outside a repository the git helpers return empty/false rather than erroring.
func TestGitOutsideRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	if sha := gitOutput("rev-parse", "HEAD"); sha != "" {
		t.Errorf("sha outside a repo = %q, want empty", sha)
	}
	if gitDirty() {
		t.Error("dirty outside a repo should be false")
	}
	if r := gitRemoteURL(); r != "" {
		t.Errorf("remote outside a repo = %q, want empty", r)
	}
}

// Credentials embedded in an http(s) remote are stripped; other forms are left
// untouched. The token must never survive into the output.
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

// remoteHostPath splits scp-style, ssh://, and https:// remotes into host and
// owner/repo, dropping the .git suffix and any port, and returns empty on junk.
func TestRemoteHostPath(t *testing.T) {
	cases := []struct{ in, host, path string }{
		{"git@github.com:acme/widgets.git", "github.com", "acme/widgets"},
		{"https://github.com/acme/widgets.git", "github.com", "acme/widgets"},
		{"https://github.com/acme/widgets", "github.com", "acme/widgets"},
		{"ssh://git@gitlab.com/grp/sub/proj.git", "gitlab.com", "grp/sub/proj"},
		{"ssh://git@example.com:2222/o/r.git", "example.com", "o/r"},
		{"git@gitlab.com:grp/sub/proj.git", "gitlab.com", "grp/sub/proj"},
		{"", "", ""},
		{"not a url", "", ""},
	}
	for _, c := range cases {
		h, p := remoteHostPath(c.in)
		if h != c.host || p != c.path {
			t.Errorf("remoteHostPath(%q) = (%q, %q), want (%q, %q)", c.in, h, p, c.host, c.path)
		}
	}
}

// httpsRepoURL builds the web URL and repoSlug the owner/repo path; neither may
// leak a credential even from an unsanitized token URL.
func TestHTTPSRepoURLAndSlug(t *testing.T) {
	cases := []struct{ remote, url, slug string }{
		{"git@github.com:acme/widgets.git", "https://github.com/acme/widgets", "acme/widgets"},
		// Even an unsanitized token URL must not leak credentials into the web URL.
		{"https://gitlab-ci-token:tok@gitlab.com/o/r.git", "https://gitlab.com/o/r", "o/r"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := httpsRepoURL(c.remote); got != c.url {
			t.Errorf("httpsRepoURL(%q) = %q, want %q", c.remote, got, c.url)
		}
		if strings.Contains(httpsRepoURL(c.remote), "@") {
			t.Errorf("httpsRepoURL(%q) leaked a credential", c.remote)
		}
		if got := repoSlug(c.remote); got != c.slug {
			t.Errorf("repoSlug(%q) = %q, want %q", c.remote, got, c.slug)
		}
	}
}
