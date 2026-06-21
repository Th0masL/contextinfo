package contextinfo

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func sampleInfo() Info {
	return Info{
		CI:      CIInfo{Detected: true, Name: "github-actions", BuildURL: "https://x/runs/1", BuildNumber: "7"},
		Git:     GitInfo{Commit: "a1b2c3", Branch: "main", Tag: "", Dirty: false, Remote: "git@github.com:o/r.git"},
		Runtime: RuntimeInfo{OS: "linux", Arch: "amd64", Hostname: "host"},
	}
}

func TestFlatJSON(t *testing.T) {
	b, err := sampleInfo().FlatJSON("")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, b)
	}
	// Flat, underscore-joined keys with no prefix; nested keys must be gone.
	if m["ci_name"] != "github-actions" {
		t.Errorf("ci_name = %v", m["ci_name"])
	}
	if _, nested := m["ci"]; nested {
		t.Error("output should be flat, found nested \"ci\" key")
	}
	// Booleans stay booleans.
	if v, ok := m["git_dirty"].(bool); !ok || v {
		t.Errorf("git_dirty = %v (%T), want false bool", m["git_dirty"], m["git_dirty"])
	}
	if _, prefixed := m["contextinfo_ci_name"]; prefixed {
		t.Error("default output must not carry a contextinfo_ prefix")
	}
}

func TestFlatJSONPrefix(t *testing.T) {
	b, _ := sampleInfo().FlatJSON("TF_VAR_")
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if _, ok := m["TF_VAR_git_commit"]; !ok {
		t.Errorf("missing TF_VAR_git_commit; got keys %v", keysOf(m))
	}
	if _, ok := m["git_commit"]; ok {
		t.Error("unprefixed git_commit should not be present when a prefix is set")
	}
}

func TestTFVarsJSONEqualsFlatJSON(t *testing.T) {
	a, _ := sampleInfo().TFVarsJSON("p_")
	b, _ := sampleInfo().FlatJSON("p_")
	if !bytes.Equal(a, b) {
		t.Errorf("TFVarsJSON and FlatJSON differ:\n%s\n---\n%s", a, b)
	}
}

func TestTFVarsHCL(t *testing.T) {
	out := sampleInfo().TFVarsHCL("")
	for _, want := range []string{
		`ci_name`,
		`= "github-actions"`,
		`git_dirty`,
		`= false`, // bare boolean, not quoted
		`runtime_os`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HCL output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "contextinfo_") {
		t.Errorf("default HCL output must not carry a contextinfo_ prefix\n%s", out)
	}
}

func TestTFVarsHCLPrefix(t *testing.T) {
	out := sampleInfo().TFVarsHCL("TF_VAR_")
	if !strings.Contains(out, "TF_VAR_git_commit") {
		t.Errorf("prefixed HCL output missing TF_VAR_git_commit\n%s", out)
	}
}

func TestTFVarsHCLEscapesInterpolation(t *testing.T) {
	info := sampleInfo()
	info.Git.Branch = `feat/${injected}`
	out := info.TFVarsHCL("")
	if !strings.Contains(out, `feat/$${injected}`) {
		t.Errorf("expected ${ to be escaped as $${, got:\n%s", out)
	}
	if strings.Contains(out, `feat/${injected}`) {
		t.Errorf("raw ${ interpolation leaked into HCL output:\n%s", out)
	}
}

func TestTFVarsHCLEscapesDirectiveAndQuotes(t *testing.T) {
	info := sampleInfo()
	info.Git.Remote = `a"b%{c}`
	out := info.TFVarsHCL("")
	if !strings.Contains(out, `a\"b%%{c}`) {
		t.Errorf("expected quote and %%{ escaping, got:\n%s", out)
	}
}

func keysOf(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
