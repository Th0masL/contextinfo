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

import "os"

// Info is the detected context as a single flat set of fields. Each field is
// resolved from the best available source: git and the OS locally, with CI
// variables taking precedence where they are more authoritative.
type Info struct {
	// Git / repository state (detected locally, augmented by CI).
	GitBranch         string `json:"git_branch"`           // current branch ("" on a tag/detached checkout)
	GitCommitSHA      string `json:"git_commit_sha"`       // full HEAD commit SHA
	GitCommitSHAShort string `json:"git_commit_sha_short"` // first 7 chars of git_commit_sha
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

	// explained always maps each field to a note of where its value came from,
	// captured during detection. explain (set by WithExplain) gates whether
	// flatten emits them as "<field>_explained" companions. Both are unexported
	// (not part of the JSON struct shape).
	explained map[string]string
	explain   bool
}

// Option configures Detect.
type Option func(*options)

// options holds the resolved configuration for a Detect call (see Option).
type options struct {
	dir      string
	checksum bool
	explain  bool
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

// WithExplain makes Detect also record, for each field, where its value came
// from. The notes surface as "<field>_explained" companions in the rendered
// output (EnvVars/FlatJSON/TFVarsHCL/Text), carrying the source text
// (variable and command names), never raw env values.
func WithExplain() Option {
	return func(o *options) { o.explain = true }
}

// Detect gathers context from the process environment and a working directory
// (the current directory by default; override with WithDir). It never fails;
// unavailable values are left empty. By default it computes files_checksum — pass
// WithoutFilesChecksum to skip that work. Detect keeps no global state and is safe
// to call concurrently for different directories.
func Detect(opts ...Option) Info {
	o := options{checksum: true}
	for _, opt := range opts {
		opt(&o)
	}
	return detect(os.Getenv, o)
}

// detect is the env-injectable core of Detect: getenv supplies CI/CD variables
// (os.Getenv in production, a fixture map in tests), while git and host state
// come from the current working directory and machine.
func detect(getenv func(string) string, o options) Info {
	ci, ciSrc := detectCI(getenv)

	sha := gitOutput(o.dir, "rev-parse", "HEAD")
	short := sha
	if len(short) > 7 {
		short = short[:7]
	}
	branch, branchSrc := gitBranch(o.dir, ci.branchHint, ciSrc["git_branch"])
	remote := gitRemoteURL(o.dir)

	info := Info{
		GitBranch:         branch,
		GitCommitSHA:      sha,
		GitCommitSHAShort: short,
		GitTag:            gitOutput(o.dir, "describe", "--tags", "--exact-match"),
		GitDirty:          gitDirty(o.dir),
		GitRepoURL:        firstNonEmpty(ci.repoURL, httpsRepoURL(remote)),
		GitRepository:     firstNonEmpty(ci.repository, repoSlug(remote)),
		Actor:             firstNonEmpty(ci.actor, osUser()),
		Event:             firstNonEmpty(ci.event, "manual"),
		CIPlatform:        ci.platform,
		CIBuildURL:        ci.buildURL,
		CIBuildNumber:     ci.buildNumber,
		CIWorkflow:        ci.workflow,
		RuntimeHostname:   hostname(),
	}
	if o.checksum {
		info.FilesChecksum = filesChecksum(o.dir)
	}
	// Provenance is captured in one pass from the values/sources already computed
	// (no re-derivation); explain only gates whether flatten emits it.
	info.explained = buildExplained(ci, ciSrc, info, branchSrc, o.checksum)
	info.explain = o.explain
	return info
}
