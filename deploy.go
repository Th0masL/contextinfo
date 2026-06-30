package contextinfo

import (
	"strconv"

	"github.com/Th0masL/contextinfo/deploy"
)

// Deploy rules map the detected context to derived variables — typically a
// deployment target such as env_name and build_type, but the set is open-ended.
// The rule model and matcher live in the deploy package; this file owns the field
// vocabulary (which detected fields a condition can match on) and the glue to
// apply rules to an Info.

// DeployRules is a set of deploy rules. Get one from the config subpackage
// (config.Config.DeployRules, parsed from a .contextinfo.yaml) or build one in
// code with the deploy package; apply it with WithDeployRules, or evaluate it
// directly with Resolve. The `=` makes this a type alias — an exact synonym for
// deploy.Rules, not a distinct type — so values of either are interchangeable
// with no conversion.
type DeployRules = deploy.Rules

// Resolve applies deploy rules to a detected Info and returns the derived
// variables (the default set with the first matching rule's set overlaid).
// Detect(WithDeployRules(...)) does this automatically and exposes the result as
// output fields; Resolve is for callers that hold an Info and rules separately.
func Resolve(rules DeployRules, info Info) map[string]string {
	vars, _ := rules.Resolve(info.lookup())
	return vars
}

// DeployVars returns the derived deploy variables Detect computed from the
// WithDeployRules/WithDeployVar options — e.g. env_name, build_type — as a fresh
// map (nil when none were supplied). It is the structured counterpart to the
// rendered output: where the four renderers fold these into formatted text,
// DeployVars hands them back as data. Unlike the stateless package-level Resolve,
// it reflects exactly what Detect produced, including any WithDeployVar overrides.
func (i Info) DeployVars() map[string]string {
	if i.derived == nil {
		return nil
	}
	out := make(map[string]string, len(i.derived))
	for k, v := range i.derived {
		out[k] = v
	}
	return out
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
	case "commit_subject", "git_commit_subject":
		return i.GitCommitSubject, true
	case "is_merge", "git_is_merge":
		return strconv.FormatBool(i.GitIsMerge), true
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
