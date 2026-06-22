package contextinfo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Outside CI, every field is resolved from git and the OS (repository/URL from
// the remote, actor from the OS user, event="manual"), and the checksum is on.
func TestDetectLocal(t *testing.T) {
	dir := newRepo(t)

	info := detect(getter(nil), options{dir: dir, checksum: true}) // not in CI

	if info.CIPlatform != "" {
		t.Errorf("ci_platform = %q, want empty (local)", info.CIPlatform)
	}
	if info.FilesChecksum == "" {
		t.Error("files_checksum empty; it should be computed by default")
	}
	if info.Event != "manual" {
		t.Errorf("event = %q, want manual", info.Event)
	}
	if info.Actor == "" {
		t.Error("actor empty; expected the local OS user")
	}
	if info.GitBranch != "main" {
		t.Errorf("git_branch = %q, want main", info.GitBranch)
	}
	if len(info.GitCommitSHA) != 40 {
		t.Errorf("git_commit_sha = %q, want 40 chars", info.GitCommitSHA)
	}
	if info.GitCommitSHAShort != info.GitCommitSHA[:7] {
		t.Errorf("git_commit_sha_short = %q, want %q", info.GitCommitSHAShort, info.GitCommitSHA[:7])
	}
	if info.GitRepository != "acme/widgets" {
		t.Errorf("git_repository = %q, want acme/widgets (from remote)", info.GitRepository)
	}
	if info.GitRepoURL != "https://github.com/acme/widgets" {
		t.Errorf("git_repo_url = %q, want https://github.com/acme/widgets", info.GitRepoURL)
	}
	if info.RuntimeHostname == "" {
		t.Error("runtime_hostname empty")
	}
}

// In CI on a detached HEAD, CI values take precedence (actor/repository/URL) and
// supply the branch the detached checkout can't report.
func TestDetectCIOverridesLocal(t *testing.T) {
	dir := newRepo(t)
	runGit(t, dir, "checkout", "--detach") // CI-like detached HEAD

	info := detect(getter(map[string]string{
		"GITHUB_ACTIONS":    "true",
		"GITHUB_SERVER_URL": "https://github.com",
		"GITHUB_REPOSITORY": "octo/proj",
		"GITHUB_RUN_ID":     "5",
		"GITHUB_RUN_NUMBER": "5",
		"GITHUB_ACTOR":      "octocat",
		"GITHUB_EVENT_NAME": "push",
		"GITHUB_WORKFLOW":   "ci",
		"GITHUB_REF_TYPE":   "branch",
		"GITHUB_REF_NAME":   "feature",
	}), options{dir: dir})

	if info.CIPlatform != "github-actions" {
		t.Errorf("ci_platform = %q", info.CIPlatform)
	}
	if info.Actor != "octocat" {
		t.Errorf("actor = %q, want octocat (CI overrides local user)", info.Actor)
	}
	if info.Event != "push" {
		t.Errorf("event = %q, want push", info.Event)
	}
	if info.GitRepository != "octo/proj" {
		t.Errorf("git_repository = %q, want CI value octo/proj", info.GitRepository)
	}
	if info.GitRepoURL != "https://github.com/octo/proj" {
		t.Errorf("git_repo_url = %q, want CI value", info.GitRepoURL)
	}
	if info.GitBranch != "feature" {
		t.Errorf("git_branch = %q, want feature (CI hint on detached HEAD)", info.GitBranch)
	}
	if info.CIBuildNumber != "5" || info.CIWorkflow != "ci" {
		t.Errorf("ci build fields wrong: number=%q workflow=%q", info.CIBuildNumber, info.CIWorkflow)
	}
}

// The JSON output is one flat object: every field at the top level, with no
// ci/git/runtime nesting.
func TestDetectJSONIsFlat(t *testing.T) {
	dir := newRepo(t)

	b, err := json.Marshal(detect(getter(nil), options{dir: dir}))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, key := range []string{
		`"git_branch"`, `"git_commit_sha"`, `"git_commit_sha_short"`, `"git_tag"`,
		`"git_dirty"`, `"files_checksum"`, `"git_repo_url"`, `"git_repository"`,
		`"actor"`, `"event"`, `"ci_platform"`, `"ci_build_url"`, `"ci_build_number"`,
		`"ci_workflow"`, `"runtime_hostname"`,
	} {
		if !strings.Contains(s, key) {
			t.Errorf("flat JSON missing %s: %s", key, s)
		}
	}
	for _, nested := range []string{`"ci":`, `"git":`, `"runtime":`} {
		if strings.Contains(s, nested) {
			t.Errorf("JSON should be flat, found nested %s: %s", nested, s)
		}
	}
}

// Detect keeps no global state, so concurrent calls for different directories
// don't interfere. Run with -race to catch data races; each result must reflect
// its own repo.
func TestDetectConcurrentDirs(t *testing.T) {
	a := newRepo(t) // origin acme/widgets
	b := newRepo(t)
	runGit(t, b, "remote", "set-url", "origin", "git@github.com:other/proj.git")
	if err := os.WriteFile(filepath.Join(b, "f.txt"), []byte("different\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, b, "commit", "-aqm", "change")

	const n = 24
	repos := []struct{ dir, slug string }{{a, "acme/widgets"}, {b, "other/proj"}}
	got := make([]Info, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i] = detect(getter(nil), options{dir: repos[i%2].dir, checksum: true})
		}()
	}
	wg.Wait()

	for i, info := range got {
		want := repos[i%2].slug
		if info.GitRepository != want {
			t.Errorf("call %d: git_repository = %q, want %q (dir isolation broke)", i, info.GitRepository, want)
		}
		if info.FilesChecksum == "" {
			t.Errorf("call %d: files_checksum empty", i)
		}
	}
	if got[0].GitCommitSHA == got[1].GitCommitSHA {
		t.Error("the two repos should have different commit SHAs")
	}
}
