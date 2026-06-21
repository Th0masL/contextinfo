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
			url, num := d.build()
			return CIInfo{Detected: true, Name: d.name, BuildURL: url, BuildNumber: num}
		}
	}
	if os.Getenv("CI") == "true" {
		return CIInfo{Detected: true, Name: "unknown"}
	}
	return CIInfo{Detected: false, Name: "local"}
}

// ciDetector recognizes one CI platform by the presence (or truthiness) of a
// marker environment variable, and knows how to extract its build URL/number.
type ciDetector struct {
	name   string
	envKey string
	truthy bool // when true, require envKey == "true"; otherwise just non-empty
	build  func() (buildURL, buildNumber string)
}

func (d ciDetector) matches() bool {
	v := os.Getenv(d.envKey)
	if d.truthy {
		return v == "true"
	}
	return v != ""
}

// ciDetectors is ordered: the first match wins.
var ciDetectors = []ciDetector{
	{"github-actions", "GITHUB_ACTIONS", true, func() (string, string) {
		server := strings.TrimRight(envOr("GITHUB_SERVER_URL", "https://github.com"), "/")
		repo, runID := os.Getenv("GITHUB_REPOSITORY"), os.Getenv("GITHUB_RUN_ID")
		url := ""
		if repo != "" && runID != "" {
			url = server + "/" + repo + "/actions/runs/" + runID
		}
		return url, os.Getenv("GITHUB_RUN_NUMBER")
	}},
	{"gitlab-ci", "GITLAB_CI", true, func() (string, string) {
		return firstNonEmpty(os.Getenv("CI_PIPELINE_URL"), os.Getenv("CI_JOB_URL")),
			firstNonEmpty(os.Getenv("CI_PIPELINE_IID"), os.Getenv("CI_PIPELINE_ID"))
	}},
	{"circleci", "CIRCLECI", true, func() (string, string) {
		return os.Getenv("CIRCLE_BUILD_URL"), os.Getenv("CIRCLE_BUILD_NUM")
	}},
	{"jenkins", "JENKINS_URL", false, func() (string, string) {
		return os.Getenv("BUILD_URL"), os.Getenv("BUILD_NUMBER")
	}},
	{"travis-ci", "TRAVIS", true, func() (string, string) {
		return os.Getenv("TRAVIS_BUILD_WEB_URL"), os.Getenv("TRAVIS_BUILD_NUMBER")
	}},
	{"buildkite", "BUILDKITE", true, func() (string, string) {
		return os.Getenv("BUILDKITE_BUILD_URL"), os.Getenv("BUILDKITE_BUILD_NUMBER")
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
