package contextinfo

import (
	"os"
	"strings"
)

// detectCI inspects environment variables to identify the CI/CD platform. The
// first matching detector wins; a generic "unknown" is reported when only the
// conventional CI=true marker is set, and "local" when nothing is detected.
func detectCI() CIInfo {
	for _, d := range ciDetectors {
		if d.matches() {
			ci := d.info()
			ci.Detected = true
			ci.Name = d.name
			return ci
		}
	}
	if os.Getenv("CI") == "true" {
		return CIInfo{Detected: true, Name: "unknown"}
	}
	return CIInfo{Detected: false, Name: "local"}
}

// ciDetector recognizes one CI platform by the presence (or truthiness) of a
// marker environment variable, and extracts that platform's CI metadata.
type ciDetector struct {
	name   string
	envKey string
	truthy bool          // when true, require envKey == "true"; otherwise just non-empty
	info   func() CIInfo // platform-specific fields (Detected/Name are set by detectCI)
}

func (d ciDetector) matches() bool {
	v := os.Getenv(d.envKey)
	if d.truthy {
		return v == "true"
	}
	return v != ""
}

// ciDetectors is ordered: the first match wins. GitHub Actions and GitLab CI
// populate the full field set; the others currently provide build URL/number
// only (their remaining fields are left empty until mapped).
var ciDetectors = []ciDetector{
	{"github-actions", "GITHUB_ACTIONS", true, func() CIInfo {
		server := strings.TrimRight(envOr("GITHUB_SERVER_URL", "https://github.com"), "/")
		repo := os.Getenv("GITHUB_REPOSITORY")
		buildURL := ""
		if runID := os.Getenv("GITHUB_RUN_ID"); repo != "" && runID != "" {
			buildURL = server + "/" + repo + "/actions/runs/" + runID
		}
		return CIInfo{
			BuildURL:    buildURL,
			BuildNumber: os.Getenv("GITHUB_RUN_NUMBER"),
			Actor:       os.Getenv("GITHUB_ACTOR"),
			Event:       os.Getenv("GITHUB_EVENT_NAME"),
			Repository:  repo,
			Workflow:    os.Getenv("GITHUB_WORKFLOW"),
			ServerURL:   server,
		}
	}},
	{"gitlab-ci", "GITLAB_CI", true, func() CIInfo {
		return CIInfo{
			BuildURL:    firstNonEmpty(os.Getenv("CI_PIPELINE_URL"), os.Getenv("CI_JOB_URL")),
			BuildNumber: firstNonEmpty(os.Getenv("CI_PIPELINE_IID"), os.Getenv("CI_PIPELINE_ID")),
			Actor:       os.Getenv("GITLAB_USER_LOGIN"),
			Event:       os.Getenv("CI_PIPELINE_SOURCE"),
			Repository:  os.Getenv("CI_PROJECT_PATH"),
			Workflow:    os.Getenv("CI_JOB_NAME"),
			ServerURL:   os.Getenv("CI_SERVER_URL"),
		}
	}},
	{"circleci", "CIRCLECI", true, func() CIInfo {
		return CIInfo{BuildURL: os.Getenv("CIRCLE_BUILD_URL"), BuildNumber: os.Getenv("CIRCLE_BUILD_NUM")}
	}},
	{"jenkins", "JENKINS_URL", false, func() CIInfo {
		return CIInfo{BuildURL: os.Getenv("BUILD_URL"), BuildNumber: os.Getenv("BUILD_NUMBER")}
	}},
	{"travis-ci", "TRAVIS", true, func() CIInfo {
		return CIInfo{BuildURL: os.Getenv("TRAVIS_BUILD_WEB_URL"), BuildNumber: os.Getenv("TRAVIS_BUILD_NUMBER")}
	}},
	{"buildkite", "BUILDKITE", true, func() CIInfo {
		return CIInfo{BuildURL: os.Getenv("BUILDKITE_BUILD_URL"), BuildNumber: os.Getenv("BUILDKITE_BUILD_NUMBER")}
	}},
}

// envOr returns the value of key, or def when key is unset/empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
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
