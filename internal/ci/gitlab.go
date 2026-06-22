package ci

// gitlabData extracts CI context from GitLab CI environment variables, plus a
// per-field map of the variable(s) each value came from (for --explain).
// See https://docs.gitlab.com/ee/ci/variables/predefined_variables.html.
func gitlabData(env func(string) string) (Data, map[string]string) {
	d := Data{
		Platform:   "gitlab-ci",
		Actor:      env("GITLAB_USER_LOGIN"),
		Event:      gitlabEvent(env),
		Repository: env("CI_PROJECT_PATH"),
		RepoURL:    env("CI_PROJECT_URL"), // already the HTTPS repo URL
		// CI_COMMIT_BRANCH is set on branch pipelines (empty on tag and
		// merge-request pipelines); on an MR the source branch is in
		// CI_MERGE_REQUEST_SOURCE_BRANCH_NAME. Neither is ever a tag name.
		BranchHint:  firstNonEmpty(env("CI_COMMIT_BRANCH"), env("CI_MERGE_REQUEST_SOURCE_BRANCH_NAME")),
		BuildURL:    firstNonEmpty(env("CI_PIPELINE_URL"), env("CI_JOB_URL")),
		BuildNumber: firstNonEmpty(env("CI_PIPELINE_IID"), env("CI_PIPELINE_ID")),
		Workflow:    env("CI_JOB_NAME"),
	}

	src := map[string]string{
		"ci_platform":     "GITLAB_CI=true",
		"actor":           "GITLAB_USER_LOGIN",
		"event":           "CI_PIPELINE_SOURCE (+CI_COMMIT_TAG), normalized",
		"git_repository":  "CI_PROJECT_PATH",
		"git_repo_url":    "CI_PROJECT_URL",
		"git_branch":      "CI_COMMIT_BRANCH / CI_MERGE_REQUEST_SOURCE_BRANCH_NAME",
		"ci_build_url":    "CI_PIPELINE_URL",
		"ci_build_number": "CI_PIPELINE_IID",
		"ci_workflow":     "CI_JOB_NAME",
	}
	return d, src
}

// gitlabEvent maps CI_PIPELINE_SOURCE to contextinfo's normalized event
// vocabulary. A tag pipeline arrives as source=push with CI_COMMIT_TAG set, so it
// normalizes to "tag" (GitLab has no distinct "release" source). Uncommon sources
// pass through their raw value.
func gitlabEvent(env func(string) string) string {
	switch s := env("CI_PIPELINE_SOURCE"); s {
	case "push":
		if env("CI_COMMIT_TAG") != "" {
			return "tag"
		}
		return "push"
	case "merge_request_event", "external_pull_request_event":
		return "pull_request"
	case "schedule":
		return "schedule"
	case "web":
		return "manual"
	default:
		return s
	}
}
