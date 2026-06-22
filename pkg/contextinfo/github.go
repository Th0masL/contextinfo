package contextinfo

import "strings"

// githubData extracts CI context from GitHub Actions environment variables.
// See https://docs.github.com/actions/learn-github-actions/variables.
func githubData(env func(string) string) ciData {
	server := strings.TrimRight(envOr(env, "GITHUB_SERVER_URL", "https://github.com"), "/")
	repo := env("GITHUB_REPOSITORY")

	d := ciData{
		platform:    "github-actions",
		actor:       env("GITHUB_ACTOR"),
		event:       env("GITHUB_EVENT_NAME"),
		repository:  repo,
		buildNumber: env("GITHUB_RUN_NUMBER"),
		workflow:    env("GITHUB_WORKFLOW"),
		branchHint:  githubBranchHint(env),
	}
	if repo != "" {
		d.repoURL = server + "/" + repo
		if runID := env("GITHUB_RUN_ID"); runID != "" {
			d.buildURL = server + "/" + repo + "/actions/runs/" + runID
		}
	}
	return d
}

// githubBranchHint returns the branch for a detached-HEAD checkout, never a tag
// name: GITHUB_HEAD_REF is the PR source branch, and GITHUB_REF_NAME holds a
// branch only when GITHUB_REF_TYPE is "branch" (it carries the tag name on
// tag/release events, and may be empty on a release event).
func githubBranchHint(env func(string) string) string {
	if h := strings.TrimSpace(env("GITHUB_HEAD_REF")); h != "" {
		return h
	}
	if env("GITHUB_REF_TYPE") == "branch" {
		return strings.TrimSpace(env("GITHUB_REF_NAME"))
	}
	return ""
}
