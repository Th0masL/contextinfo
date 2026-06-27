// Package contextinfo detects the execution context of a process: the git state
// of the working directory, the user/event/repository behind the run, and (when
// present) the CI/CD platform.
//
// Detection is local-first: branch, commit, tag, dirty state, repository URL,
// actor, and event are derived from git and the OS, so contextinfo behaves the
// same whether or not it runs in CI. CI variables only augment or override those
// values where they are more authoritative — the branch when HEAD is detached,
// the triggering user, the repository slug, the build URL, and so on.
//
// It has no external dependencies and is safe to call anywhere — detection
// failures (no git, not in CI, ...) yield empty fields rather than errors.
package contextinfo

import (
	"context"
	"os"

	"github.com/Th0masL/contextinfo/internal/ci"
	"github.com/Th0masL/contextinfo/internal/git"
	"github.com/Th0masL/contextinfo/internal/scm"
)

// Info is the detected context as a single flat set of fields. Each field is
// resolved from the best available source: git and the OS locally, with CI
// variables taking precedence where they are more authoritative.
type Info struct {
	// Git / repository state (detected locally, augmented by CI).
	GitBranch         string `json:"git_branch"`           // current branch ("" on a tag/detached checkout)
	GitCommitSHA      string `json:"git_commit_sha"`       // full HEAD commit SHA
	GitCommitSHAShort string `json:"git_commit_sha_short"` // first 7 chars of git_commit_sha
	GitCommitSubject  string `json:"git_commit_subject"`   // HEAD commit subject (first line); user-editable, treat as a hint
	GitIsMerge        bool   `json:"git_is_merge"`         // HEAD is a merge commit (2+ parents); structural, reliable
	GitTag            string `json:"git_tag"`              // tag pointing at HEAD ("" if none)
	GitDirty          bool   `json:"git_dirty"`            // working tree has uncommitted changes
	FilesChecksum     string `json:"files_checksum"`       // SHA-256 of non-ignored working-dir files ("" if disabled)
	GitRepoURL        string `json:"git_repo_url"`         // HTTPS web URL of the repository
	GitRepository     string `json:"git_repository"`       // "owner/repo" slug
	Actor             string `json:"actor"`                // CI user that triggered the run, else local OS user
	Event             string `json:"event"`                // normalized trigger: push, tag, pull_request, release, schedule, or manual

	// CI/CD platform (empty when not running in CI).
	CIPlatform    string `json:"ci_platform"`     // "github-actions", "gitlab-ci", "circleci", "unknown", or "" locally
	CIBuildURL    string `json:"ci_build_url"`    // URL of the current build/pipeline
	CIBuildNumber string `json:"ci_build_number"` // build/pipeline number
	CIWorkflow    string `json:"ci_workflow"`     // workflow or job name

	// Host runtime.
	RuntimeHostname string `json:"runtime_hostname"` // os.Hostname()

	// derived holds deploy variables computed from the context by deploy rules
	// (and/or explicit overrides) — e.g. env_name, build_type. It is empty unless
	// WithDeployRules/WithDeployVar are used. These render as additional output
	// fields after the detected ones.
	derived map[string]string

	// explained maps each field to a note of where its value came from, captured
	// during detection. It is always populated; rendering with
	// RenderOptions.Explain decides whether to emit the "<field>_explained"
	// companions. Unexported (not part of the JSON struct shape).
	explained map[string]string
}

// Option configures Detect.
type Option func(*options)

// options holds the resolved configuration for a Detect call (see Option).
type options struct {
	ctx        context.Context // bounds the git subprocesses (nil → Background)
	dir        string
	checksum   bool
	deploy     DeployRules       // deploy rules to apply (see WithDeployRules)
	hasDeploy  bool              // whether deploy rules were supplied
	deployVars map[string]string // explicit deploy-var overrides (highest precedence)
}

// WithoutFilesChecksum disables computing files_checksum. By default Detect computes it,
// which reads every non-ignored file in the working directory; disable it when
// detection must stay cheap on a very large tree.
func WithoutFilesChecksum() Option {
	return func(o *options) { o.checksum = false }
}

// WithDir sets the directory to inspect. The default (empty) is the process's
// current working directory. Git runs in this directory and detection holds no
// global state, so Detect may be called concurrently for different directories.
func WithDir(dir string) Option {
	return func(o *options) { o.dir = dir }
}

// WithDeployRules applies a set of deploy rules to the detected context. Each
// rule maps a condition over the detected fields to a set of variables (such as
// env_name and build_type); the first matching rule wins, merged over the
// default. The results render as additional output fields. Rules are usually
// loaded from a .contextinfo.yaml via the config subpackage.
func WithDeployRules(r DeployRules) Option {
	return func(o *options) {
		o.deploy = r
		o.hasDeploy = true
	}
}

// WithDeployVar forces a derived deploy variable to value, overriding whatever
// the deploy rules would set for that key. The CLI uses this for --env-name and
// --build-type.
func WithDeployVar(key, value string) Option {
	return func(o *options) {
		if o.deployVars == nil {
			o.deployVars = map[string]string{}
		}
		o.deployVars[key] = value
	}
}

// Detect gathers context from the process environment and a working directory
// (the current directory by default; override with WithDir). It never fails;
// unavailable values are left empty. By default it computes files_checksum — pass
// WithoutFilesChecksum to skip that work. Detect keeps no global state and is safe
// to call concurrently for different directories.
func Detect(opts ...Option) Info {
	return DetectContext(context.Background(), opts...)
}

// DetectContext is Detect with a caller-supplied context that bounds the git
// subprocesses, so a long-running embedder can cancel or time out detection (a
// cancelled context yields empty git-derived fields, never a panic).
func DetectContext(ctx context.Context, opts ...Option) Info {
	o := options{ctx: ctx, checksum: true}
	for _, opt := range opts {
		opt(&o)
	}
	return detect(os.Getenv, o)
}

// detect is the env-injectable core of Detect: getenv supplies CI/CD variables
// (os.Getenv in production, a fixture map in tests), while git and host state
// come from the current working directory and machine.
func detect(getenv func(string) string, o options) Info {
	ctx := o.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	cid, ciSrc := ci.Detect(getenv)

	// One probe decides whether to do any git work: outside a repo the other git
	// calls would all fail, so we skip them (and the branch comes from the CI hint
	// alone). This also distinguishes "not a repo" from "empty repo" for --explain.
	inRepo := git.InRepo(ctx, o.dir)

	var sha, short, tag, subject, remote, branch, branchSrc string
	dirty, isMerge := false, false
	if inRepo {
		var parents int
		sha, parents, subject = git.Commit(ctx, o.dir)
		isMerge = parents >= 2
		short = sha
		if len(short) > 7 {
			short = short[:7]
		}
		tag = git.Output(ctx, o.dir, "describe", "--tags", "--exact-match")
		dirty = git.Dirty(ctx, o.dir)
		remote = git.RemoteURL(ctx, o.dir)
		branch, branchSrc = git.Branch(ctx, o.dir, cid.BranchHint, ciSrc["git_branch"])
	} else if cid.BranchHint != "" {
		branch, branchSrc = cid.BranchHint, ciSrc["git_branch"]
	} else {
		branch, branchSrc = "", "none (not a git repository)"
	}

	info := Info{
		GitBranch:         branch,
		GitCommitSHA:      sha,
		GitCommitSHAShort: short,
		GitCommitSubject:  subject,
		GitIsMerge:        isMerge,
		GitTag:            tag,
		GitDirty:          dirty,
		GitRepoURL:        firstNonEmpty(cid.RepoURL, scm.HTTPSURL(remote)),
		GitRepository:     firstNonEmpty(cid.Repository, scm.Slug(remote)),
		Actor:             firstNonEmpty(cid.Actor, osUser()),
		Event:             firstNonEmpty(cid.Event, "manual"),
		CIPlatform:        cid.Platform,
		CIBuildURL:        cid.BuildURL,
		CIBuildNumber:     cid.BuildNumber,
		CIWorkflow:        cid.Workflow,
		RuntimeHostname:   hostname(),
	}
	if inRepo && o.checksum {
		info.FilesChecksum = git.Checksum(ctx, o.dir)
	}
	// Provenance is captured in one pass from the values/sources already computed
	// (no re-derivation); RenderOptions.Explain later gates whether it is emitted.
	info.explained = buildExplained(cid, ciSrc, info, branchSrc, inRepo, o.checksum)

	// Derived deploy variables: apply rules (default, then first matching rule),
	// then explicit overrides win. Provenance is recorded into explained so the
	// "<key>_explained" companions cover these too.
	if o.hasDeploy || len(o.deployVars) > 0 {
		vars, src := o.deploy.Resolve(info.lookup())
		for k, v := range o.deployVars {
			vars[k] = v
			src[k] = "deploy: explicit override"
		}
		info.derived = vars
		for k, s := range src {
			info.explained[k] = s
		}
	}
	return info
}
