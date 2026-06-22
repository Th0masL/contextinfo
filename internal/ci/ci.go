// Package ci detects the CI/CD platform from environment variables and extracts
// its context. Only platforms whose environment has been verified against real
// output are recognized by name — GitHub Actions, GitLab CI, and CircleCI. It is
// env-injectable (Detect takes a getenv func) for testing, and decoupled from the
// core package; the core merges this with locally-detected git/OS values.
package ci

// Data holds the CI/CD-provided context for one platform. Any field may be empty.
type Data struct {
	Platform    string // "github-actions", "gitlab-ci", "circleci", "unknown", or "" locally
	Actor       string // triggering user/login
	Event       string // normalized trigger (push, tag, pull_request, release, schedule, manual)
	Repository  string // "owner/repo" slug
	RepoURL     string // HTTPS repository URL
	BranchHint  string // branch for a detached-HEAD checkout (never a tag name)
	BuildURL    string // current build/pipeline URL
	BuildNumber string // build/pipeline number
	Workflow    string // workflow or job name
}

// Detect identifies the CI/CD platform via getenv and returns its context plus a
// per-field map of source labels (keyed by Info field name) describing which env
// var(s) each value came from, so the core's --explain can report provenance
// without re-deriving it. A bare CI=true marker yields platform "unknown";
// otherwise an empty Data is returned, indicating a local (non-CI) run.
func Detect(getenv func(string) string) (Data, map[string]string) {
	switch {
	case getenv("GITHUB_ACTIONS") == "true":
		return githubData(getenv)
	case getenv("GITLAB_CI") == "true":
		return gitlabData(getenv)
	case getenv("CIRCLECI") == "true":
		return circleCIData(getenv)
	case getenv("CI") == "true":
		return Data{Platform: "unknown"}, map[string]string{"ci_platform": "CI=true"}
	default:
		return Data{}, nil
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
