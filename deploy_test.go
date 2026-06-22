package contextinfo

import (
	"strings"
	"testing"

	"github.com/Th0masL/contextinfo/deploy"
)

func mustGlob(t *testing.T, g string) deploy.Pattern {
	t.Helper()
	p, err := deploy.GlobPattern(g)
	if err != nil {
		t.Fatalf("glob %q: %v", g, err)
	}
	return p
}

func branchRule(t *testing.T, glob string, set map[string]string) deploy.Rule {
	return deploy.Rule{
		If:  deploy.Cond{Fields: []deploy.FieldMatch{{Field: "branch", Patterns: []deploy.Pattern{mustGlob(t, glob)}}}},
		Set: set,
	}
}

// Every output field is matchable by its output name; git_* fields also by a
// short alias. Both must resolve to the same value.
func TestFieldValueAliases(t *testing.T) {
	i := Info{GitBranch: "main", GitTag: "v1", GitRepository: "o/r", GitDirty: true}
	pairs := [][2]string{
		{"branch", "git_branch"},
		{"tag", "git_tag"},
		{"repository", "git_repository"},
		{"dirty", "git_dirty"},
	}
	for _, p := range pairs {
		short, _ := fieldValue(i, p[0])
		full, ok := fieldValue(i, p[1])
		if !ok || short != full {
			t.Errorf("%q and %q disagree: %q vs %q", p[0], p[1], short, full)
		}
	}
	if v, _ := fieldValue(i, "dirty"); v != "true" {
		t.Errorf("dirty = %q, want \"true\"", v)
	}
}

func TestIsMatchField(t *testing.T) {
	for _, f := range []string{"branch", "git_branch", "tag", "event", "repository", "git_repository", "ci_platform", "files_checksum"} {
		if !IsMatchField(f) {
			t.Errorf("%q should be a match field", f)
		}
	}
	if IsMatchField("branche") {
		t.Error("a typo should not be a valid match field")
	}
}

// detect applies rules and exposes the result; explicit overrides win; the
// derived var renders in the output.
func TestDetectAppliesDeployRules(t *testing.T) {
	rules := deploy.Rules{
		Rules:   []deploy.Rule{branchRule(t, "main", map[string]string{"env_name": "prod"})},
		Default: map[string]string{"env_name": "dev"},
	}
	// A GitHub push to main: the ref hint supplies branch=main even outside a repo.
	env := getter(map[string]string{
		"GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "push",
		"GITHUB_REF_TYPE": "branch", "GITHUB_REF_NAME": "main",
	})
	info := detect(env, options{dir: t.TempDir(), deploy: rules, hasDeploy: true})
	if info.derived["env_name"] != "prod" {
		t.Errorf("env_name = %q, want prod", info.derived["env_name"])
	}
	js, _ := info.FlatJSON(RenderOptions{})
	if !strings.Contains(string(js), `"env_name": "prod"`) {
		t.Errorf("derived var missing from JSON:\n%s", js)
	}

	info2 := detect(env, options{dir: t.TempDir(), deploy: rules, hasDeploy: true, deployVars: map[string]string{"env_name": "forced"}})
	if info2.derived["env_name"] != "forced" {
		t.Errorf("override env_name = %q, want forced", info2.derived["env_name"])
	}
}

// Resolve applies rules to an Info directly (the public helper).
func TestResolve(t *testing.T) {
	rules := deploy.Rules{
		Rules:   []deploy.Rule{branchRule(t, "release/*", map[string]string{"env_name": "staging"})},
		Default: map[string]string{"env_name": "dev"},
	}
	if got := Resolve(rules, Info{GitBranch: "release/9"}); got["env_name"] != "staging" {
		t.Errorf("release/9 -> %v, want staging", got)
	}
	if got := Resolve(rules, Info{GitBranch: "main"}); got["env_name"] != "dev" {
		t.Errorf("main -> %v, want default dev", got)
	}
}
