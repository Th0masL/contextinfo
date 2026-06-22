package contextinfo_test

import (
	"fmt"

	"github.com/Th0masL/contextinfo"
)

// Detect resolves the run context from git, the OS, and (in CI) the platform.
// Detection never fails: unavailable values are simply empty.
func ExampleDetect() {
	info := contextinfo.Detect()
	fmt.Println(info.GitRepository, info.GitBranch, info.Event)
}

// Rendering is separate from detection: each method takes a RenderOptions, so the
// same Info can be rendered with or without a prefix (and with or without the
// "<field>_explained" companions). EnvVars emits shell NAME=value lines — strings
// are single-quoted, booleans are bare.
func ExampleInfo_EnvVars() {
	info := contextinfo.Info{GitBranch: "main", Event: "push"}
	fmt.Print(info.EnvVars(contextinfo.RenderOptions{Prefix: "TF_VAR_"}))
	// Output:
	// TF_VAR_git_branch='main'
	// TF_VAR_git_commit_sha=''
	// TF_VAR_git_commit_sha_short=''
	// TF_VAR_git_tag=''
	// TF_VAR_git_dirty=false
	// TF_VAR_files_checksum=''
	// TF_VAR_git_repo_url=''
	// TF_VAR_git_repository=''
	// TF_VAR_actor=''
	// TF_VAR_event='push'
	// TF_VAR_ci_platform=''
	// TF_VAR_ci_build_url=''
	// TF_VAR_ci_build_number=''
	// TF_VAR_ci_workflow=''
	// TF_VAR_runtime_hostname=''
}
