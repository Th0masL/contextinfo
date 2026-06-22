package contextinfo

import (
	"strings"
	"testing"
)

// WithExplain adds a "<field>_explained" companion to each field (with the source
// note) without changing the underlying values.
func TestExplainLocal(t *testing.T) {
	dir := newRepo(t)

	plain := detect(getter(nil), options{dir: dir, checksum: true})
	info := detect(getter(nil), options{dir: dir, checksum: true, explain: true})

	// Underlying values are unchanged by explain.
	if info.GitBranch != plain.GitBranch || info.GitCommitSHA != plain.GitCommitSHA ||
		info.FilesChecksum != plain.FilesChecksum || info.GitRepository != plain.GitRepository {
		t.Error("WithExplain changed field values")
	}

	// The source notes match the local resolution.
	for field, want := range map[string]string{
		"git_branch":     "git symbolic-ref --short HEAD",
		"git_commit_sha": "git rev-parse HEAD",
		"git_repository": "git remote origin",
		"actor":          "OS user",
		"event":          "default (not in CI)",
		"ci_platform":    "not in CI",
	} {
		if got := info.explained[field]; got != want {
			t.Errorf("%s source = %q, want %q", field, got, want)
		}
	}

	// Rendered output carries both the field and its _explained companion.
	out := info.EnvVars("")
	for _, want := range []string{
		"git_branch='main'",
		"git_branch_explained='git symbolic-ref --short HEAD'",
		"event='manual'",
		"event_explained='default (not in CI)'",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("envvar output missing %q\n%s", want, out)
		}
	}
}

// In CI, the _explained notes name the provider variables that supplied each
// value (and the CI hint that supplied the branch on a detached HEAD).
func TestExplainCISources(t *testing.T) {
	dir := newRepo(t)
	runGit(t, dir, "checkout", "--detach")

	info := detect(getter(map[string]string{
		"GITHUB_ACTIONS":    "true",
		"GITHUB_SERVER_URL": "https://github.com",
		"GITHUB_REPOSITORY": "octo/proj",
		"GITHUB_RUN_ID":     "5",
		"GITHUB_RUN_NUMBER": "5",
		"GITHUB_ACTOR":      "octocat",
		"GITHUB_EVENT_NAME": "push",
		"GITHUB_REF_TYPE":   "branch",
		"GITHUB_REF_NAME":   "feature",
	}), options{dir: dir, checksum: true, explain: true})

	for field, want := range map[string]string{
		"actor":           "GITHUB_ACTOR",
		"git_repository":  "GITHUB_REPOSITORY",
		"ci_platform":     "GITHUB_ACTIONS=true",
		"ci_build_number": "GITHUB_RUN_NUMBER",
	} {
		if got := info.explained[field]; got != want {
			t.Errorf("%s source = %q, want %q", field, got, want)
		}
	}
	if got := info.explained["git_branch"]; !strings.Contains(got, "GITHUB_REF_NAME") {
		t.Errorf("git_branch source = %q, want it to mention GITHUB_REF_NAME (detached-HEAD hint)", got)
	}
}
