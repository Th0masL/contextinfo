package contextinfo

import "strings"

// githubData extracts CI context from GitHub Actions environment variables, plus
// a per-field map of the variable(s) each value came from (for --explain).
// See https://docs.github.com/actions/learn-github-actions/variables.
func githubData(env func(string) string) (ciData, map[string]string) {
	server := strings.TrimRight(envOr(env, "GITHUB_SERVER_URL", "https://github.com"), "/")
	repo := env("GITHUB_REPOSITORY")

	d := ciData{
		platform:    "github-actions",
		actor:       env("GITHUB_ACTOR"),
		event:       githubEvent(env),
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

	src := map[string]string{
		"ci_platform":     "GITHUB_ACTIONS=true",
		"actor":           "GITHUB_ACTOR",
		"event":           "GITHUB_EVENT_NAME (+GITHUB_REF_TYPE), normalized",
		"git_repository":  "GITHUB_REPOSITORY",
		"git_repo_url":    "GITHUB_SERVER_URL + GITHUB_REPOSITORY",
		"git_branch":      "GITHUB_REF_NAME/GITHUB_HEAD_REF",
		"ci_build_url":    "GITHUB_SERVER_URL + GITHUB_REPOSITORY + GITHUB_RUN_ID",
		"ci_build_number": "GITHUB_RUN_NUMBER",
		"ci_workflow":     "GITHUB_WORKFLOW",
	}
	return d, src
}

// githubEvent maps GITHUB_EVENT_NAME to contextinfo's normalized event
// vocabulary (push, tag, pull_request, release, schedule, manual). A tag push
// arrives as event_name=push with ref_type=tag, so it normalizes to "tag".
// Uncommon events pass through their raw GitHub name.
func githubEvent(env func(string) string) string {
	switch e := env("GITHUB_EVENT_NAME"); e {
	case "push":
		if env("GITHUB_REF_TYPE") == "tag" {
			return "tag"
		}
		return "push"
	case "pull_request", "pull_request_target":
		return "pull_request"
	case "release":
		return "release"
	case "schedule":
		return "schedule"
	case "workflow_dispatch", "repository_dispatch":
		return "manual"
	default:
		return e
	}
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
