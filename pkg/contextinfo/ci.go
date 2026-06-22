package contextinfo

// ciData holds the CI/CD-provided context for one platform. Any field may be
// empty; detect merges these with locally-detected git/OS values.
type ciData struct {
	platform    string // "github-actions", "gitlab-ci", "unknown", or "" locally
	actor       string // triggering user/login
	event       string // event/source (push, release, schedule, ...)
	repository  string // "owner/repo" slug
	repoURL     string // HTTPS repository URL
	branchHint  string // branch for a detached-HEAD checkout (never a tag name)
	buildURL    string // current build/pipeline URL
	buildNumber string // build/pipeline number
	workflow    string // workflow or job name
}

// detectCI identifies the CI/CD platform from environment variables (read via
// getenv) and extracts its context. Only platforms whose environment has been
// verified against real output are recognized by name — GitHub Actions, GitLab
// CI, and CircleCI. A bare CI=true marker yields platform "unknown"; otherwise an
// empty ciData is returned, indicating a local (non-CI) run.
func detectCI(getenv func(string) string) ciData {
	switch {
	case getenv("GITHUB_ACTIONS") == "true":
		return githubData(getenv)
	case getenv("GITLAB_CI") == "true":
		return gitlabData(getenv)
	case getenv("CIRCLECI") == "true":
		return circleCIData(getenv)
	case getenv("CI") == "true":
		return ciData{platform: "unknown"}
	default:
		return ciData{}
	}
}

// envOr returns getenv(key), or def when it is unset/empty.
func envOr(getenv func(string) string, key, def string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return def
}

// firstNonEmpty returns the first non-empty argument, or "".
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
