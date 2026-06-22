package contextinfo

// circleCIData extracts CI context from CircleCI environment variables.
// See https://circleci.com/docs/variables/#built-in-environment-variables.
//
// CircleCI exposes no single "event" variable, so it is derived from the build
// ref: a tag build sets CIRCLE_TAG (and leaves CIRCLE_BRANCH empty), a branch
// build sets CIRCLE_BRANCH. CIRCLE_REPOSITORY_URL is not always populated, so the
// repository slug comes from CIRCLE_PROJECT_USERNAME/CIRCLE_PROJECT_REPONAME and
// the URL falls back to the local git remote (see detect) when env has none.
func circleCIData(env func(string) string) ciData {
	repo := ""
	if owner, name := env("CIRCLE_PROJECT_USERNAME"), env("CIRCLE_PROJECT_REPONAME"); owner != "" && name != "" {
		repo = owner + "/" + name
	}

	event := ""
	switch {
	case env("CIRCLE_TAG") != "":
		event = "tag"
	case env("CIRCLE_BRANCH") != "":
		event = "push"
	}

	return ciData{
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
}
