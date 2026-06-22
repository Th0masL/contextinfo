package contextinfo

import (
	"strconv"

	"github.com/Th0masL/contextinfo/internal/deploy"
)

// Deploy rules map the detected context to derived variables — typically a
// deployment target such as env_name and build_type, but the set is open-ended.
// The rule model and matcher live in the internal/deploy package (so they stay
// out of the public API); this file owns the field vocabulary (which detected
// fields a condition can match on) and the glue to apply rules to an Info.

// DeployRules is an opaque set of deploy rules. Build one with the config
// subpackage (config.Config.DeployRules) and apply it with WithDeployRules, or
// evaluate it directly with Resolve.
type DeployRules = deploy.Rules

// Resolve applies deploy rules to a detected Info and returns the derived
// variables (the default set with the first matching rule's set overlaid).
// Detect(WithDeployRules(...)) does this automatically and exposes the result as
// output fields; Resolve is for callers that hold an Info and rules separately.
func Resolve(rules DeployRules, info Info) map[string]string {
	vars, _ := rules.Resolve(info.lookup())
	return vars
}

// lookup adapts an Info to the deploy engine's field-resolution function.
func (i Info) lookup() deploy.Lookup {
	return func(name string) (string, bool) { return fieldValue(i, name) }
}

// fieldValue resolves a condition field name to the corresponding Info value.
// Each field is addressable by its output name (e.g. git_branch) and, for the
// git_* fields, by a short alias (branch). Booleans render as "true"/"false".
// The bool result reports whether the name is a known field.
func fieldValue(i Info, name string) (string, bool) {
	switch name {
	case "branch", "git_branch":
		return i.GitBranch, true
	case "commit_sha", "git_commit_sha":
		return i.GitCommitSHA, true
	case "commit_sha_short", "git_commit_sha_short":
		return i.GitCommitSHAShort, true
	case "tag", "git_tag":
		return i.GitTag, true
	case "dirty", "git_dirty":
		return strconv.FormatBool(i.GitDirty), true
	case "files_checksum":
		return i.FilesChecksum, true
	case "repo_url", "git_repo_url":
		return i.GitRepoURL, true
	case "repository", "git_repository":
		return i.GitRepository, true
	case "actor":
		return i.Actor, true
	case "event":
		return i.Event, true
	case "ci_platform":
		return i.CIPlatform, true
	case "ci_build_url":
		return i.CIBuildURL, true
	case "ci_build_number":
		return i.CIBuildNumber, true
	case "ci_workflow":
		return i.CIWorkflow, true
	case "runtime_hostname":
		return i.RuntimeHostname, true
	}
	return "", false
}

// IsMatchField reports whether name is a field that deploy-rule conditions can
// match on (any output field name, plus the short git_* aliases). The config
// parser uses it to reject typos at load time.
func IsMatchField(name string) bool {
	_, ok := fieldValue(Info{}, name)
	return ok
}
