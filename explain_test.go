package contextinfo

import (
	"strings"
	"testing"
)

// Provenance is always captured during detection; rendering with
// RenderOptions.Explain decides whether to emit the "<field>_explained"
// companions, without changing the underlying values.
func TestExplainLocal(t *testing.T) {
	dir := newRepo(t)
	info := detect(getter(nil), options{dir: dir, checksum: true})

	// The source notes match the local resolution.
	for field, want := range map[string]string{
		"git_branch":     "git symbolic-ref --short HEAD",
		"git_commit_sha": "git log -1 --format=%H",
		"git_repository": "git remote origin",
		"actor":          "OS user",
		"event":          "default (not in CI)",
		"ci_platform":    "not in CI",
	} {
		if got := info.explained[field]; got != want {
			t.Errorf("%s source = %q, want %q", field, got, want)
		}
	}

	// A plain render omits the companions; an Explain render adds them while the
	// field values stay identical.
	if plain := info.EnvVars(RenderOptions{}); strings.Contains(plain, "_explained") {
		t.Errorf("plain render leaked _explained companions:\n%s", plain)
	}
	out := info.EnvVars(RenderOptions{Explain: true})
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
	}), options{dir: dir, checksum: true})

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
