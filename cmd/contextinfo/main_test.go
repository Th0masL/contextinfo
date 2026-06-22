package main

import (
	"encoding/json"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildCLI compiles the contextinfo binary into a temp dir and returns its path.
// It exercises the real executable end-to-end (flag parsing, output, exit codes),
// which the library unit tests don't cover.
func buildCLI(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "contextinfo")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

// runCLI runs the binary and returns its stdout and exit code.
func runCLI(t *testing.T, bin string, args ...string) (stdout string, code int) {
	t.Helper()
	var so strings.Builder
	cmd := exec.Command(bin, args...)
	cmd.Stdout = &so
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if ok := asExitError(err, &ee); ok {
			return so.String(), ee.ExitCode()
		}
		t.Fatalf("run %v: %v", args, err)
	}
	return so.String(), 0
}

// asExitError reports whether err is an *exec.ExitError and, if so, stores it in
// target (so runCLI can read the process exit code).
func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

// --version prints a non-empty version and exits 0.
func TestCLIVersion(t *testing.T) {
	bin := buildCLI(t)
	out, code := runCLI(t, bin, "--version")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.TrimSpace(out) == "" {
		t.Error("--version printed nothing")
	}
}

// With no flags the output is envvar lines, not JSON.
func TestCLIDefaultIsEnvVar(t *testing.T) {
	bin := buildCLI(t)
	out, code := runCLI(t, bin) // no args
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "git_commit_sha=") {
		t.Errorf("default output should be envvar lines, got:\n%s", out)
	}
	if strings.Contains(out, "{") {
		t.Errorf("default output should not be JSON:\n%s", out)
	}
}

// --prefix is applied to the envvar names.
func TestCLIEnvVarPrefix(t *testing.T) {
	bin := buildCLI(t)
	out, code := runCLI(t, bin, "--format=envvar", "--prefix", "TF_VAR_")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "TF_VAR_git_commit_sha=") {
		t.Errorf("envvar --prefix not applied:\n%s", out)
	}
	// Booleans are bare, strings are single-quoted.
	if !strings.Contains(out, "TF_VAR_git_dirty=") {
		t.Errorf("missing TF_VAR_git_dirty:\n%s", out)
	}
}

// --format=json emits a flat JSON object with no ci/git/runtime nesting.
func TestCLIJSON(t *testing.T) {
	bin := buildCLI(t)
	out, code := runCLI(t, bin, "--format=json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	var info map[string]any
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	for _, key := range []string{"git_branch", "git_commit_sha", "git_repository", "event", "ci_platform", "runtime_hostname"} {
		if _, ok := info[key]; !ok {
			t.Errorf("flat JSON missing %q key\n%s", key, out)
		}
	}
	for _, nested := range []string{"ci", "git", "runtime"} {
		if _, ok := info[nested]; ok {
			t.Errorf("JSON should be flat, found nested %q key\n%s", nested, out)
		}
	}
}

// --help prints usage with the flags, format list, and examples.
func TestCLIHelp(t *testing.T) {
	bin := buildCLI(t)
	// --help/-h prints usage and exits 0; flag sends it to stderr, so capture both.
	out, err := exec.Command(bin, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("--help exited non-zero: %v\n%s", err, out)
	}
	for _, want := range []string{"Usage", "Formats", "Examples", "envvar", "-format", "-prefix", "TF_VAR_"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("--help output missing %q\n%s", want, out)
		}
	}
}

// --format=text prints aligned key/value rows.
func TestCLIText(t *testing.T) {
	bin := buildCLI(t)
	out, code := runCLI(t, bin, "--format=text")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"git_branch", "git_repository", "runtime_hostname"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q\n%s", want, out)
		}
	}
}

// tfvars / tfvars-json emit Terraform variables (HCL and JSON respectively).
func TestCLITFVars(t *testing.T) {
	bin := buildCLI(t)

	// Default: no prefix.
	hcl, code := runCLI(t, bin, "--format=tfvars")
	if code != 0 {
		t.Fatalf("tfvars: exit = %d, want 0", code)
	}
	if !strings.Contains(hcl, "runtime_hostname") || !strings.Contains(hcl, " = ") {
		t.Errorf("tfvars (HCL) output missing expected variable assignment\n%s", hcl)
	}
	if strings.Contains(hcl, "contextinfo_") {
		t.Errorf("tfvars should have no prefix by default\n%s", hcl)
	}

	js, code := runCLI(t, bin, "--format=tfvars-json")
	if code != 0 {
		t.Fatalf("tfvars-json: exit = %d, want 0", code)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(js), &m); err != nil {
		t.Fatalf("tfvars-json is not valid JSON: %v\n%s", err, js)
	}
	if _, ok := m["runtime_hostname"]; !ok {
		t.Errorf("tfvars-json missing runtime_hostname\n%s", js)
	}
}

// --prefix is applied to the tfvars variable names.
func TestCLIPrefix(t *testing.T) {
	bin := buildCLI(t)
	out, code := runCLI(t, bin, "--format=tfvars", "--prefix", "TF_VAR_")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "TF_VAR_git_commit_sha") {
		t.Errorf("--prefix not applied; expected TF_VAR_git_commit_sha\n%s", out)
	}
}

// git_checksum is populated by default and suppressed by --no-checksum.
func TestCLIChecksumDefaultAndDisable(t *testing.T) {
	bin := buildCLI(t)

	// Default: git_checksum is computed (the test runs inside this git repo).
	out, code := runCLI(t, bin, "--format=json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if s, _ := m["git_checksum"].(string); s == "" {
		t.Errorf("git_checksum should be populated by default\n%s", out)
	}

	// --no-checksum leaves it empty.
	out2, code := runCLI(t, bin, "--no-checksum", "--format=json")
	if code != 0 {
		t.Fatalf("--no-checksum exit = %d, want 0", code)
	}
	var m2 map[string]any
	if err := json.Unmarshal([]byte(out2), &m2); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out2)
	}
	if s, _ := m2["git_checksum"].(string); s != "" {
		t.Errorf("git_checksum should be empty with --no-checksum, got %q", s)
	}
}

// An unknown --format exits non-zero.
func TestCLIUnknownFormatFails(t *testing.T) {
	bin := buildCLI(t)
	_, code := runCLI(t, bin, "--format=bogus")
	if code == 0 {
		t.Error("expected non-zero exit for an unknown format")
	}
}
