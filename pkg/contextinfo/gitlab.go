package contextinfo

// gitlabData extracts CI context from GitLab CI environment variables.
// See https://docs.gitlab.com/ee/ci/variables/predefined_variables.html.
func gitlabData(env func(string) string) ciData {
	return ciData{
		platform:   "gitlab-ci",
		actor:      env("GITLAB_USER_LOGIN"),
		event:      env("CI_PIPELINE_SOURCE"),
		repository: env("CI_PROJECT_PATH"),
		repoURL:    env("CI_PROJECT_URL"), // already the HTTPS repo URL
		// CI_COMMIT_BRANCH is set only on branch pipelines (empty on tag pipelines),
		// so it never mislabels a tag checkout as a branch.
		branchHint:  env("CI_COMMIT_BRANCH"),
		buildURL:    firstNonEmpty(env("CI_PIPELINE_URL"), env("CI_JOB_URL")),
		buildNumber: firstNonEmpty(env("CI_PIPELINE_IID"), env("CI_PIPELINE_ID")),
		workflow:    env("CI_JOB_NAME"),
	}
}
