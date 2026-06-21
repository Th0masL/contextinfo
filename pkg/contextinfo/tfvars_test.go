package contextinfo

import (
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

func TestTFVarsJSON(t *testing.T) {
	b, err := sampleInfo().TFVarsJSON()
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, b)
	}
	if m["contextinfo_ci_name"] != "github-actions" {
		t.Errorf("ci_name = %v", m["contextinfo_ci_name"])
	}
	// Booleans must be JSON booleans, not strings.
	if v, ok := m["contextinfo_git_dirty"].(bool); !ok || v {
		t.Errorf("git_dirty = %v (%T), want false bool", m["contextinfo_git_dirty"], m["contextinfo_git_dirty"])
	}
	if m["contextinfo_runtime_os"] != "linux" {
		t.Errorf("runtime_os = %v", m["contextinfo_runtime_os"])
	}
}

func TestTFVarsHCL(t *testing.T) {
	out := sampleInfo().TFVarsHCL()
	for _, want := range []string{
		`contextinfo_ci_name`,
		`= "github-actions"`,
		`contextinfo_git_dirty`,
		`= false`, // bare boolean, not quoted
		`contextinfo_runtime_os`,
		`= "linux"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HCL output missing %q\n%s", want, out)
		}
	}
}

func TestTFVarsHCLEscapesInterpolation(t *testing.T) {
	info := sampleInfo()
	info.Git.Branch = `feat/${injected}` // a value that could break naive HCL
	out := info.TFVarsHCL()

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
	out := info.TFVarsHCL()
	if !strings.Contains(out, `a\"b%%{c}`) {
		t.Errorf("expected quote and %%{ escaping, got:\n%s", out)
	}
}
