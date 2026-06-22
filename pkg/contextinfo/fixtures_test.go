package contextinfo

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadEnvFixture parses a "KEY=value" environment dump (one per line, '#'
// comments and blanks ignored) into a getenv-style lookup.
func loadEnvFixture(t *testing.T, path string) func(string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m := map[string]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			m[k] = v
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return func(k string) string { return m[k] }
}

// TestGoldenFixtures runs detectCI over real CI environment dumps captured in
// testdata/env (see the test-printenv sandbox repos). It pins the discriminating
// fields (platform/event/branchHint) and requires the run-specific fields to be
// populated, so a regression in the per-provider env mapping is caught.
func TestGoldenFixtures(t *testing.T) {
	type want struct {
		platform   string
		event      string
		branchHint string
	}
	cases := map[string]want{
		"github_push_main.txt":                          {"github-actions", "push", "main"},
		"github_push_development.txt":                   {"github-actions", "push", "development"},
		"github_tag-created-during-release_v1.0.0.txt":  {"github-actions", "tag", ""},
		"github_release_v1.0.0.txt":                     {"github-actions", "release", ""},
		"gitlab_push_main.txt":                          {"gitlab-ci", "push", "main"},
		"gitlab_push_development.txt":                   {"gitlab-ci", "push", "development"},
		"gitlab_release_v1.0.0.txt":                     {"gitlab-ci", "tag", ""},
		"gitlab_tag-created-without-release_v1.0.1.txt": {"gitlab-ci", "tag", ""},
		"circleci_push_main.txt":                        {"circleci", "push", "main"},
		"circleci_release-on-github_v1.0.2.txt":         {"circleci", "tag", ""},
		"github_schedule_on-main-branch.txt":            {"github-actions", "schedule", "main"},
		"gitlab_schedule_on-main-branch.txt":            {"gitlab-ci", "schedule", "main"},
		// The "merge-pr" captures are the post-merge PUSH to main (the printenv jobs
		// don't run on pull_request / merge_request pipelines), so the event is
		// "push", not "pull_request" — they don't add pull_request coverage.
		"github_merge-pr_to-main-branch.txt":                         {"github-actions", "push", "main"},
		"gitlab_merge-pr_to-main-branch.txt":                         {"gitlab-ci", "push", "main"},
		"circleci_merge-pr-triggered-from-github_to-main-branch.txt": {"circleci", "push", "main"},
	}

	dir := filepath.Join("testdata", "env")
	files, err := filepath.Glob(filepath.Join(dir, "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no fixtures found in %s", dir)
	}

	seen := map[string]bool{}
	for _, path := range files {
		name := filepath.Base(path)
		w, ok := cases[name]
		if !ok {
			t.Errorf("fixture %s has no expectation in the test table", name)
			continue
		}
		seen[name] = true
		t.Run(name, func(t *testing.T) {
			d, _ := detectCI(loadEnvFixture(t, path))
			if d.platform != w.platform {
				t.Errorf("platform = %q, want %q", d.platform, w.platform)
			}
			if d.event != w.event {
				t.Errorf("event = %q, want %q", d.event, w.event)
			}
			if d.branchHint != w.branchHint {
				t.Errorf("branchHint = %q, want %q", d.branchHint, w.branchHint)
			}
			// Run-specific values: require them to be present for a CI fixture.
			// repoURL is omitted — it's provider-dependent (CircleCI's
			// CIRCLE_REPOSITORY_URL can be empty; full Detect() then falls back to
			// the local git remote). The per-provider unit tests assert it where it
			// applies.
			for label, v := range map[string]string{
				"actor":       d.actor,
				"repository":  d.repository,
				"buildURL":    d.buildURL,
				"buildNumber": d.buildNumber,
				"workflow":    d.workflow,
			} {
				if v == "" {
					t.Errorf("%s is empty; expected a value from the fixture", label)
				}
			}
			if strings.Contains(d.repoURL, "@") {
				t.Errorf("repoURL contains '@' (possible leaked credential): %q", d.repoURL)
			}
		})
	}
	for name := range cases {
		if !seen[name] {
			t.Errorf("expected fixture %s not found on disk", name)
		}
	}
}
