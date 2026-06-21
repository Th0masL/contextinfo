// Package contextinfo detects the execution context of a process: the CI/CD
// platform, git state of the working directory, and host runtime.
//
// It has no external dependencies and is safe to call anywhere — detection
// failures (no git, not in CI, ...) yield empty fields rather than errors.
package contextinfo

// Info is the full detected context returned by Detect.
type Info struct {
	CI      CIInfo      `json:"ci"`
	Git     GitInfo     `json:"git"`
	Runtime RuntimeInfo `json:"runtime"`
}

// CIInfo describes the detected CI/CD environment. Only GitHub Actions and
// GitLab CI are recognized by name and have their environments verified; any
// other CI is reported as "unknown" (and the detailed fields are left empty)
// rather than guessing at unverified variables.
type CIInfo struct {
	Detected    bool   `json:"detected"`     // whether a CI environment was recognized
	Name        string `json:"name"`         // "github-actions", "gitlab-ci", "unknown", or "local"
	BuildURL    string `json:"build_url"`    // URL of the current build/pipeline, if known
	BuildNumber string `json:"build_number"` // build/pipeline number, if known
	Actor       string `json:"actor"`        // user/login that triggered the run
	Event       string `json:"event"`        // event/source that triggered the run (push, tag, ...)
	Repository  string `json:"repository"`   // owner/repo slug (join with ServerURL for the repo URL)
	Workflow    string `json:"workflow"`     // workflow or job name
	ServerURL   string `json:"server_url"`   // CI server base URL (e.g. https://github.com)
}

// GitInfo describes the git state of the current working directory.
type GitInfo struct {
	Commit string `json:"commit"` // HEAD commit SHA
	Branch string `json:"branch"` // current branch (CI env fallback when detached)
	Tag    string `json:"tag"`    // tag pointing at HEAD, if any
	Dirty  bool   `json:"dirty"`  // whether the working tree has uncommitted changes
	Remote string `json:"remote"` // origin remote URL (embedded credentials stripped)
}

// RuntimeInfo describes the host runtime.
type RuntimeInfo struct {
	OS       string `json:"os"`       // GOOS
	Arch     string `json:"arch"`     // GOARCH
	Hostname string `json:"hostname"` // os.Hostname()
}

// Detect gathers CI, git, and runtime context from the current process and
// working directory. It never fails; unavailable values are left empty.
func Detect() Info {
	return Info{
		CI:      detectCI(),
		Git:     detectGit(),
		Runtime: detectRuntime(),
	}
}
