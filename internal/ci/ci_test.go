package ci

import (
	"reflect"
	"sort"
	"testing"
)

// getter returns a getenv-style lookup backed by m (missing keys yield "").
func getter(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// second returns the source-label map from a per-provider detector's two results.
func second(_ Data, src map[string]string) map[string]string { return src }

// Every provider must expose the same set of source-label keys (one per
// CI-augmented field). This fails loudly if a provider's label table drifts —
// e.g. a new field is wired into one provider but not the others, which would
// otherwise just leave a silently-empty "<field>_explained" companion.
func TestCISourceLabelKeysInLockstep(t *testing.T) {
	want := []string{
		"actor", "ci_build_number", "ci_build_url", "ci_platform",
		"ci_workflow", "event", "git_branch", "git_repo_url", "git_repository",
	}
	get := getter(nil) // the label tables are static; the env values don't matter
	providers := map[string]map[string]string{
		"github":   second(githubData(get)),
		"gitlab":   second(gitlabData(get)),
		"circleci": second(circleCIData(get)),
	}
	for name, src := range providers {
		got := make([]string, 0, len(src))
		for k := range src {
			got = append(got, k)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s source-label keys = %v,\n             want %v", name, got, want)
		}
	}
}

// With no CI markers set, detection reports a local (empty-platform) run.
func TestDetectLocal(t *testing.T) {
	if d, _ := Detect(getter(nil)); d.Platform != "" {
		t.Errorf("local: platform = %q, want empty", d.Platform)
	}
}

// A bare CI=true from an unrecognized platform is reported as "unknown".
func TestDetectGenericUnknown(t *testing.T) {
	if d, _ := Detect(getter(map[string]string{"CI": "true"})); d.Platform != "unknown" {
		t.Errorf("platform = %q, want unknown", d.Platform)
	}
}

// When both a known platform marker and the generic CI marker are set, the
// specific platform must win over the "unknown" fallback.
func TestDetectPrecedenceGitHubBeforeGeneric(t *testing.T) {
	d, _ := Detect(getter(map[string]string{"CI": "true", "GITHUB_ACTIONS": "true"}))
	if d.Platform != "github-actions" {
		t.Errorf("platform = %q, want github-actions (specific platform must win)", d.Platform)
	}
}

// A full GitHub Actions environment maps to every Data field.
func TestGitHubData(t *testing.T) {
	d, _ := Detect(getter(map[string]string{
		"GITHUB_ACTIONS":    "true",
		"GITHUB_SERVER_URL": "https://github.com",
		"GITHUB_REPOSITORY": "org/repo",
		"GITHUB_RUN_ID":     "123",
		"GITHUB_RUN_NUMBER": "7",
		"GITHUB_ACTOR":      "octocat",
		"GITHUB_EVENT_NAME": "push",
		"GITHUB_WORKFLOW":   "deploy",
		"GITHUB_REF_TYPE":   "branch",
		"GITHUB_REF_NAME":   "main",
	}))
	want := Data{
		Platform:    "github-actions",
		Actor:       "octocat",
		Event:       "push",
		Repository:  "org/repo",
		RepoURL:     "https://github.com/org/repo",
		BranchHint:  "main",
		BuildURL:    "https://github.com/org/repo/actions/runs/123",
		BuildNumber: "7",
		Workflow:    "deploy",
	}
	if d != want {
		t.Errorf("githubData mismatch\n got: %+v\nwant: %+v", d, want)
	}
}

// A tag push (event_name=push + ref_type=tag) normalizes event to "tag" and has
// no branch hint (GITHUB_REF_NAME holds the tag, not a branch).
func TestGitHubDataTagEventHasNoBranchHint(t *testing.T) {
	d, _ := Detect(getter(map[string]string{
		"GITHUB_ACTIONS":    "true",
		"GITHUB_EVENT_NAME": "push",
		"GITHUB_REF_TYPE":   "tag",
		"GITHUB_REF_NAME":   "v1.2.3",
	}))
	if d.Event != "tag" {
		t.Errorf("event = %q, want tag", d.Event)
	}
	if d.BranchHint != "" {
		t.Errorf("tag event: branchHint = %q, want empty", d.BranchHint)
	}
}

// A pull-request build's branch hint is the source branch, not the merge ref.
func TestGitHubDataPullRequestUsesHeadRef(t *testing.T) {
	d, _ := Detect(getter(map[string]string{
		"GITHUB_ACTIONS":  "true",
		"GITHUB_HEAD_REF": "feature/x",
		"GITHUB_REF_TYPE": "branch",
		"GITHUB_REF_NAME": "main",
	}))
	if d.BranchHint != "feature/x" {
		t.Errorf("PR: branchHint = %q, want feature/x", d.BranchHint)
	}
}

// A full GitLab CI environment maps to every Data field.
func TestGitLabData(t *testing.T) {
	d, _ := Detect(getter(map[string]string{
		"GITLAB_CI":          "true",
		"GITLAB_USER_LOGIN":  "tux",
		"CI_PIPELINE_SOURCE": "push",
		"CI_PROJECT_PATH":    "org/repo",
		"CI_PROJECT_URL":     "https://gitlab.com/org/repo",
		"CI_COMMIT_BRANCH":   "main",
		"CI_PIPELINE_URL":    "https://gitlab.com/org/repo/-/pipelines/99",
		"CI_PIPELINE_IID":    "9",
		"CI_JOB_NAME":        "deploy",
	}))
	want := Data{
		Platform:    "gitlab-ci",
		Actor:       "tux",
		Event:       "push",
		Repository:  "org/repo",
		RepoURL:     "https://gitlab.com/org/repo",
		BranchHint:  "main",
		BuildURL:    "https://gitlab.com/org/repo/-/pipelines/99",
		BuildNumber: "9",
		Workflow:    "deploy",
	}
	if d != want {
		t.Errorf("gitlabData mismatch\n got: %+v\nwant: %+v", d, want)
	}
}

// A full CircleCI branch build maps to every Data field (event derived "push").
func TestCircleCIData(t *testing.T) {
	d, _ := Detect(getter(map[string]string{
		"CI":                      "true", // CircleCI sets both; CIRCLECI must win
		"CIRCLECI":                "true",
		"CIRCLE_USERNAME":         "octocat",
		"CIRCLE_PROJECT_USERNAME": "acme",
		"CIRCLE_PROJECT_REPONAME": "widgets",
		"CIRCLE_REPOSITORY_URL":   "git@github.com:acme/widgets.git",
		"CIRCLE_BRANCH":           "main",
		"CIRCLE_BUILD_URL":        "https://app.circleci.com/jobs/x/1",
		"CIRCLE_BUILD_NUM":        "2",
		"CIRCLE_JOB":              "printenv",
	}))
	want := Data{
		Platform:    "circleci",
		Actor:       "octocat",
		Event:       "push",
		Repository:  "acme/widgets",
		RepoURL:     "https://github.com/acme/widgets",
		BranchHint:  "main",
		BuildURL:    "https://app.circleci.com/jobs/x/1",
		BuildNumber: "2",
		Workflow:    "printenv",
	}
	if d != want {
		t.Errorf("circleCIData mismatch\n got: %+v\nwant: %+v", d, want)
	}
}

// A CircleCI tag build derives event="tag" and leaves the branch hint empty.
func TestCircleCIDataTagBuild(t *testing.T) {
	d, _ := Detect(getter(map[string]string{
		"CIRCLECI":   "true",
		"CIRCLE_TAG": "v1.0.2",
	}))
	if d.Event != "tag" {
		t.Errorf("event = %q, want tag", d.Event)
	}
	if d.BranchHint != "" {
		t.Errorf("branchHint = %q, want empty on a tag build", d.BranchHint)
	}
}

// A CircleCI PR build sets CIRCLE_PULL_REQUEST; event normalizes to "pull_request".
func TestCircleCIDataPullRequest(t *testing.T) {
	d, _ := Detect(getter(map[string]string{
		"CIRCLECI":            "true",
		"CIRCLE_BRANCH":       "feature/x",
		"CIRCLE_PULL_REQUEST": "https://github.com/acme/widgets/pull/7",
	}))
	if d.Event != "pull_request" {
		t.Errorf("event = %q, want pull_request", d.Event)
	}
}

// A GitLab tag pipeline is source=push with CI_COMMIT_TAG set and
// CI_COMMIT_BRANCH unset: event normalizes to "tag" and there is no branch hint.
func TestGitLabDataTagPipelineHasNoBranchHint(t *testing.T) {
	d, _ := Detect(getter(map[string]string{
		"GITLAB_CI":          "true",
		"CI_PIPELINE_SOURCE": "push",
		"CI_COMMIT_TAG":      "v1.2.3",
		"CI_COMMIT_REF_NAME": "v1.2.3",
	}))
	if d.Event != "tag" {
		t.Errorf("event = %q, want tag", d.Event)
	}
	if d.BranchHint != "" {
		t.Errorf("tag pipeline: branchHint = %q, want empty", d.BranchHint)
	}
}

// A GitLab merge-request pipeline has CI_COMMIT_BRANCH empty, so the branch hint
// falls back to the MR source branch (CI_MERGE_REQUEST_SOURCE_BRANCH_NAME).
func TestGitLabDataMergeRequest(t *testing.T) {
	d, _ := Detect(getter(map[string]string{
		"GITLAB_CI":                           "true",
		"CI_PIPELINE_SOURCE":                  "merge_request_event",
		"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME": "feature/x",
	}))
	if d.Event != "pull_request" {
		t.Errorf("event = %q, want pull_request", d.Event)
	}
	if d.BranchHint != "feature/x" {
		t.Errorf("branchHint = %q, want feature/x (MR source branch)", d.BranchHint)
	}
}
