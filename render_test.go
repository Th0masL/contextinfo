package contextinfo

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// flatten() is hand-maintained in lockstep with the Info struct; this fails if a
// field is added/removed/reordered in one but not the other, so the explicit list
// can't silently drift. (flatten feeds envvar/json/tfvars.)
func TestFlattenMatchesStructTags(t *testing.T) {
	rt := reflect.TypeOf(Info{})
	var tags []string
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("json")
		if c := strings.IndexByte(tag, ','); c >= 0 {
			tag = tag[:c]
		}
		if tag != "" && tag != "-" {
			tags = append(tags, tag)
		}
	}
	var keys []string
	for _, p := range (Info{}).flatten(RenderOptions{}) {
		keys = append(keys, p.key)
	}
	if !reflect.DeepEqual(tags, keys) {
		t.Errorf("flatten() keys drifted from Info json tags:\n  struct : %v\n  flatten: %v", tags, keys)
	}
}

// sampleInfo returns a fully-populated Info used by the rendering tests.
func sampleInfo() Info {
	return Info{
		GitBranch:         "main",
		GitCommitSHA:      "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		GitCommitSHAShort: "a1b2c3d",
		GitTag:            "",
		GitDirty:          false,
		FilesChecksum:     "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		GitRepoURL:        "https://github.com/o/r",
		GitRepository:     "o/r",
		Actor:             "octocat",
		Event:             "push",
		CIPlatform:        "github-actions",
		CIBuildURL:        "https://x/runs/1",
		CIBuildNumber:     "7",
		CIWorkflow:        "deploy",
		RuntimeHostname:   "host",
	}
}

// FlatJSON (no prefix) is a flat object — no nesting, booleans stay booleans.
func TestFlatJSON(t *testing.T) {
	b, err := sampleInfo().FlatJSON(RenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, b)
	}
	if m["git_branch"] != "main" {
		t.Errorf("git_branch = %v", m["git_branch"])
	}
	if _, nested := m["git"]; nested {
		t.Error("output should be flat, found nested \"git\" key")
	}
	// Booleans stay booleans.
	if v, ok := m["git_dirty"].(bool); !ok || v {
		t.Errorf("git_dirty = %v (%T), want false bool", m["git_dirty"], m["git_dirty"])
	}
	if _, prefixed := m["contextinfo_git_branch"]; prefixed {
		t.Error("default output must not carry a contextinfo_ prefix")
	}
}

// A prefix is applied to every key, and the unprefixed form disappears.
func TestFlatJSONPrefix(t *testing.T) {
	b, _ := sampleInfo().FlatJSON(RenderOptions{Prefix: "TF_VAR_"})
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if _, ok := m["TF_VAR_git_commit_sha"]; !ok {
		t.Errorf("missing TF_VAR_git_commit_sha; got keys %v", keysOf(m))
	}
	if _, ok := m["git_commit_sha"]; ok {
		t.Error("unprefixed git_commit_sha should not be present when a prefix is set")
	}
}

// HCL output uses bare booleans and quoted strings, with no prefix by default.
func TestTFVarsHCL(t *testing.T) {
	out := sampleInfo().TFVarsHCL(RenderOptions{})
	for _, want := range []string{
		`git_branch`,
		`= "main"`,
		`git_dirty`,
		`= false`, // bare boolean, not quoted
		`git_repository`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HCL output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "contextinfo_") {
		t.Errorf("default HCL output must not carry a contextinfo_ prefix\n%s", out)
	}
}

// A prefix is applied to the HCL variable names.
func TestTFVarsHCLPrefix(t *testing.T) {
	out := sampleInfo().TFVarsHCL(RenderOptions{Prefix: "TF_VAR_"})
	if !strings.Contains(out, "TF_VAR_git_commit_sha") {
		t.Errorf("prefixed HCL output missing TF_VAR_git_commit_sha\n%s", out)
	}
}

// ${...} is escaped to $${...} so an untrusted value can't inject a Terraform
// interpolation.
func TestTFVarsHCLEscapesInterpolation(t *testing.T) {
	info := sampleInfo()
	info.GitBranch = `feat/${injected}`
	out := info.TFVarsHCL(RenderOptions{})
	if !strings.Contains(out, `feat/$${injected}`) {
		t.Errorf("expected ${ to be escaped as $${, got:\n%s", out)
	}
	if strings.Contains(out, `feat/${injected}`) {
		t.Errorf("raw ${ interpolation leaked into HCL output:\n%s", out)
	}
}

// Double quotes and %{...} directives are escaped inside HCL strings.
func TestTFVarsHCLEscapesDirectiveAndQuotes(t *testing.T) {
	info := sampleInfo()
	info.GitRepoURL = `a"b%{c}`
	out := info.TFVarsHCL(RenderOptions{})
	if !strings.Contains(out, `a\"b%%{c}`) {
		t.Errorf("expected quote and %%{ escaping, got:\n%s", out)
	}
}

// envvar output: bare booleans and single-quoted strings (shell-safe).
func TestEnvVars(t *testing.T) {
	out := sampleInfo().EnvVars(RenderOptions{})
	if !strings.Contains(out, "git_dirty=false") {
		t.Errorf("boolean should be bare:\n%s", out)
	}
	if !strings.Contains(out, "git_repo_url='https://github.com/o/r'") {
		t.Errorf("string should be single-quoted:\n%s", out)
	}
}

// keysOf returns the keys of m, used to make assertion failures readable.
func keysOf(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
