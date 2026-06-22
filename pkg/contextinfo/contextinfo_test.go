package contextinfo

import (
	"encoding/json"
	"strings"
	"testing"
)

// Outside CI, every field is resolved from git and the OS (repository/URL from
// the remote, actor from the OS user, event="manual"), and the checksum is on.
func TestDetectLocal(t *testing.T) {
	dir := newRepo(t)
	t.Chdir(dir)

	info := detect(getter(nil), options{checksum: true}) // not in CI

	if info.CIPlatform != "" {
		t.Errorf("ci_platform = %q, want empty (local)", info.CIPlatform)
	}
	if info.GitChecksum == "" {
		t.Error("git_checksum empty; it should be computed by default")
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
	t.Chdir(dir)
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
	}), options{})

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
	t.Chdir(dir)

	b, err := json.Marshal(detect(getter(nil), options{}))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, key := range []string{
		`"git_branch"`, `"git_commit_sha"`, `"git_commit_sha_short"`, `"git_tag"`,
		`"git_dirty"`, `"git_checksum"`, `"git_repo_url"`, `"git_repository"`,
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
