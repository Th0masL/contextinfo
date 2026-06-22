package contextinfo

// circleCIData extracts CI context from CircleCI environment variables, plus a
// per-field map of the variable(s) each value came from (for --explain).
// See https://circleci.com/docs/variables/#built-in-environment-variables.
//
// CircleCI exposes no single "event" variable, so the normalized event is
// derived from the build ref: CIRCLE_TAG -> "tag", CIRCLE_PULL_REQUEST ->
// "pull_request", else CIRCLE_BRANCH -> "push". CIRCLE_REPOSITORY_URL is not
// always populated, so the repository slug comes from
// CIRCLE_PROJECT_USERNAME/CIRCLE_PROJECT_REPONAME and the URL falls back to the
// local git remote (see detect) when env has none.
func circleCIData(env func(string) string) (ciData, map[string]string) {
	repo := ""
	if owner, name := env("CIRCLE_PROJECT_USERNAME"), env("CIRCLE_PROJECT_REPONAME"); owner != "" && name != "" {
		repo = owner + "/" + name
	}

	event := ""
	switch {
	case env("CIRCLE_TAG") != "":
		event = "tag"
	case env("CIRCLE_PULL_REQUEST") != "":
		event = "pull_request"
	case env("CIRCLE_BRANCH") != "":
		event = "push"
	}

	d := ciData{
		platform:    "circleci",
		actor:       env("CIRCLE_USERNAME"),
		event:       event,
		repository:  repo,
		repoURL:     httpsRepoURL(env("CIRCLE_REPOSITORY_URL")), // "" when unset
		branchHint:  env("CIRCLE_BRANCH"),                       // empty on tag builds
		buildURL:    env("CIRCLE_BUILD_URL"),
		buildNumber: env("CIRCLE_BUILD_NUM"),
		workflow:    env("CIRCLE_JOB"),
	}

	src := map[string]string{
		"ci_platform":     "CIRCLECI=true",
		"actor":           "CIRCLE_USERNAME",
		"event":           "CIRCLE_TAG/CIRCLE_PULL_REQUEST/CIRCLE_BRANCH, normalized",
		"git_repository":  "CIRCLE_PROJECT_USERNAME + CIRCLE_PROJECT_REPONAME",
		"git_repo_url":    "CIRCLE_REPOSITORY_URL",
		"git_branch":      "CIRCLE_BRANCH",
		"ci_build_url":    "CIRCLE_BUILD_URL",
		"ci_build_number": "CIRCLE_BUILD_NUM",
		"ci_workflow":     "CIRCLE_JOB",
	}
	return d, src
}
