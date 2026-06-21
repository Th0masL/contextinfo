package contextinfo

import "testing"

// clearCI blanks every CI marker this package inspects, so a test starts from a
// known "not in CI" baseline even when the test itself runs inside CI.
func clearCI(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI",
		"JENKINS_URL", "TRAVIS", "BUILDKITE",
	} {
		t.Setenv(k, "")
	}
}

func TestDetectCILocal(t *testing.T) {
	clearCI(t)
	ci := detectCI()
	if ci.Detected || ci.Name != "local" {
		t.Errorf("expected local, got %+v", ci)
	}
}

func TestDetectGitHubActions(t *testing.T) {
	clearCI(t)
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_SERVER_URL", "https://github.com")
	t.Setenv("GITHUB_REPOSITORY", "org/repo")
	t.Setenv("GITHUB_RUN_ID", "123")
	t.Setenv("GITHUB_RUN_NUMBER", "7")
	t.Setenv("GITHUB_ACTOR", "octocat")
	t.Setenv("GITHUB_EVENT_NAME", "push")
	t.Setenv("GITHUB_WORKFLOW", "deploy")

	ci := detectCI()
	if !ci.Detected || ci.Name != "github-actions" {
		t.Fatalf("got %+v", ci)
	}
	if want := "https://github.com/org/repo/actions/runs/123"; ci.BuildURL != want {
		t.Errorf("build_url = %q, want %q", ci.BuildURL, want)
	}
	if ci.BuildNumber != "7" {
		t.Errorf("build_number = %q, want 7", ci.BuildNumber)
	}
	if ci.Actor != "octocat" || ci.Event != "push" || ci.Repository != "org/repo" ||
		ci.Workflow != "deploy" || ci.ServerURL != "https://github.com" {
		t.Errorf("extended fields wrong: %+v", ci)
	}
}

func TestDetectGitLab(t *testing.T) {
	clearCI(t)
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PIPELINE_URL", "https://gitlab.com/org/repo/-/pipelines/99")
	t.Setenv("CI_PIPELINE_IID", "9")
	t.Setenv("GITLAB_USER_LOGIN", "tux")
	t.Setenv("CI_PIPELINE_SOURCE", "push")
	t.Setenv("CI_PROJECT_PATH", "org/repo")
	t.Setenv("CI_JOB_NAME", "deploy")
	t.Setenv("CI_SERVER_URL", "https://gitlab.com")

	ci := detectCI()
	if ci.Name != "gitlab-ci" {
		t.Errorf("name = %q, want gitlab-ci", ci.Name)
	}
	if ci.BuildURL == "" || ci.BuildNumber != "9" {
		t.Errorf("got %+v", ci)
	}
	if ci.Actor != "tux" || ci.Event != "push" || ci.Repository != "org/repo" ||
		ci.Workflow != "deploy" || ci.ServerURL != "https://gitlab.com" {
		t.Errorf("extended fields wrong: %+v", ci)
	}
}

func TestDetectCircleCI(t *testing.T) {
	clearCI(t)
	t.Setenv("CIRCLECI", "true")
	t.Setenv("CIRCLE_BUILD_URL", "https://circleci.com/gh/org/repo/42")
	t.Setenv("CIRCLE_BUILD_NUM", "42")

	ci := detectCI()
	if ci.Name != "circleci" || ci.BuildNumber != "42" {
		t.Errorf("got %+v", ci)
	}
}

func TestDetectJenkins(t *testing.T) {
	clearCI(t)
	t.Setenv("JENKINS_URL", "https://jenkins.example.com/")
	t.Setenv("BUILD_URL", "https://jenkins.example.com/job/x/5/")
	t.Setenv("BUILD_NUMBER", "5")

	ci := detectCI()
	if ci.Name != "jenkins" || ci.BuildNumber != "5" {
		t.Errorf("got %+v", ci)
	}
}

func TestDetectGenericCI(t *testing.T) {
	clearCI(t)
	t.Setenv("CI", "true")

	ci := detectCI()
	if !ci.Detected || ci.Name != "unknown" {
		t.Errorf("got %+v", ci)
	}
}

func TestDetectPrecedenceGitHubBeforeGeneric(t *testing.T) {
	clearCI(t)
	// Both the generic marker and a specific platform are set; the specific
	// platform must win.
	t.Setenv("CI", "true")
	t.Setenv("GITHUB_ACTIONS", "true")

	if ci := detectCI(); ci.Name != "github-actions" {
		t.Errorf("name = %q, want github-actions", ci.Name)
	}
}
