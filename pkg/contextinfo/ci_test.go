package contextinfo

import "testing"

// getter returns a getenv-style lookup backed by m (missing keys yield "").
func getter(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// With no CI markers set, detection reports a local (empty-platform) run.
func TestDetectCILocal(t *testing.T) {
	if d := detectCI(getter(nil)); d.platform != "" {
		t.Errorf("local: platform = %q, want empty", d.platform)
	}
}

// A bare CI=true from an unrecognized platform is reported as "unknown".
func TestDetectCIGenericUnknown(t *testing.T) {
	if d := detectCI(getter(map[string]string{"CI": "true"})); d.platform != "unknown" {
		t.Errorf("platform = %q, want unknown", d.platform)
	}
}

// When both a known platform marker and the generic CI marker are set, the
// specific platform must win over the "unknown" fallback.
func TestDetectCIPrecedenceGitHubBeforeGeneric(t *testing.T) {
	d := detectCI(getter(map[string]string{"CI": "true", "GITHUB_ACTIONS": "true"}))
	if d.platform != "github-actions" {
		t.Errorf("platform = %q, want github-actions (specific platform must win)", d.platform)
	}
}

// A full GitHub Actions environment maps to every ciData field.
func TestGitHubData(t *testing.T) {
	d := detectCI(getter(map[string]string{
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
	want := ciData{
		platform:    "github-actions",
		actor:       "octocat",
		event:       "push",
		repository:  "org/repo",
		repoURL:     "https://github.com/org/repo",
		branchHint:  "main",
		buildURL:    "https://github.com/org/repo/actions/runs/123",
		buildNumber: "7",
		workflow:    "deploy",
	}
	if d != want {
		t.Errorf("githubData mismatch\n got: %+v\nwant: %+v", d, want)
	}
}

// A tag/release build must not produce a branch hint.
func TestGitHubDataTagEventHasNoBranchHint(t *testing.T) {
	// On a tag/release event GITHUB_REF_NAME holds the tag, not a branch.
	d := detectCI(getter(map[string]string{
		"GITHUB_ACTIONS":  "true",
		"GITHUB_REF_TYPE": "tag",
		"GITHUB_REF_NAME": "v1.2.3",
	}))
	if d.branchHint != "" {
		t.Errorf("tag event: branchHint = %q, want empty", d.branchHint)
	}
}

// A pull-request build's branch hint is the source branch, not the merge ref.
func TestGitHubDataPullRequestUsesHeadRef(t *testing.T) {
	// In a PR run GITHUB_REF_NAME is the merge ref; the source branch is HEAD_REF.
	d := detectCI(getter(map[string]string{
		"GITHUB_ACTIONS":  "true",
		"GITHUB_HEAD_REF": "feature/x",
		"GITHUB_REF_TYPE": "branch",
		"GITHUB_REF_NAME": "main",
	}))
	if d.branchHint != "feature/x" {
		t.Errorf("PR: branchHint = %q, want feature/x", d.branchHint)
	}
}

// A full GitLab CI environment maps to every ciData field.
func TestGitLabData(t *testing.T) {
	d := detectCI(getter(map[string]string{
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
	want := ciData{
		platform:    "gitlab-ci",
		actor:       "tux",
		event:       "push",
		repository:  "org/repo",
		repoURL:     "https://gitlab.com/org/repo",
		branchHint:  "main",
		buildURL:    "https://gitlab.com/org/repo/-/pipelines/99",
		buildNumber: "9",
		workflow:    "deploy",
	}
	if d != want {
		t.Errorf("gitlabData mismatch\n got: %+v\nwant: %+v", d, want)
	}
}

// A full CircleCI branch build maps to every ciData field (event derived "push").
func TestCircleCIData(t *testing.T) {
	d := detectCI(getter(map[string]string{
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
	want := ciData{
		platform:    "circleci",
		actor:       "octocat",
		event:       "push",
		repository:  "acme/widgets",
		repoURL:     "https://github.com/acme/widgets",
		branchHint:  "main",
		buildURL:    "https://app.circleci.com/jobs/x/1",
		buildNumber: "2",
		workflow:    "printenv",
	}
	if d != want {
		t.Errorf("circleCIData mismatch\n got: %+v\nwant: %+v", d, want)
	}
}

// A CircleCI tag build derives event="tag" and leaves the branch hint empty.
func TestCircleCIDataTagBuild(t *testing.T) {
	// On a tag build CIRCLE_TAG is set and CIRCLE_BRANCH is empty; event is
	// derived as "tag" and the branch hint must stay empty.
	d := detectCI(getter(map[string]string{
		"CIRCLECI":   "true",
		"CIRCLE_TAG": "v1.0.2",
	}))
	if d.event != "tag" {
		t.Errorf("event = %q, want tag", d.event)
	}
	if d.branchHint != "" {
		t.Errorf("branchHint = %q, want empty on a tag build", d.branchHint)
	}
}

// A GitLab tag pipeline leaves CI_COMMIT_BRANCH unset, so no branch hint.
func TestGitLabDataTagPipelineHasNoBranchHint(t *testing.T) {
	// CI_COMMIT_BRANCH is unset on tag pipelines (only CI_COMMIT_TAG/REF_NAME).
	d := detectCI(getter(map[string]string{
		"GITLAB_CI":          "true",
		"CI_COMMIT_TAG":      "v1.2.3",
		"CI_COMMIT_REF_NAME": "v1.2.3",
	}))
	if d.branchHint != "" {
		t.Errorf("tag pipeline: branchHint = %q, want empty", d.branchHint)
	}
}
