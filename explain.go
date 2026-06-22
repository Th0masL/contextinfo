package contextinfo

// buildExplained assembles the per-field source notes used for the
// "<field>_explained" companions. It takes the values and sources already
// computed during detection (ci + its source labels, the resolved info, the
// branch source, whether we're in a repo, and whether the checksum ran), so it
// never re-runs git or re-reads the environment. The CI source labels come from
// the per-provider detectors (ciSrc), so there is no duplicated var-name table to
// drift. The notes name variables and commands, not their contents, so explaining
// never exposes secrets.
func buildExplained(ci ciData, ciSrc map[string]string, info Info, branchSrc string, inRepo, checksum bool) map[string]string {
	tag := "git describe --tags --exact-match"
	if info.GitTag == "" {
		if inRepo {
			tag += " (no tag at HEAD)"
		} else {
			tag = "none (not a git repository)"
		}
	}

	dirty := "git status --porcelain (empty)"
	switch {
	case !inRepo:
		dirty = "none (not a git repository)"
	case info.GitDirty:
		dirty = "git status --porcelain (non-empty)"
	}

	cksum := "sha256 over git ls-files -z --cached --others --exclude-standard"
	switch {
	case !checksum:
		cksum = "disabled (--no-files-checksum)"
	case !inRepo:
		cksum = "none (not a git repository)"
	}

	platform := "not in CI"
	if ci.platform != "" {
		platform = ciSrc["ci_platform"]
	}

	return map[string]string{
		"git_branch":           branchSrc,
		"git_commit_sha":       gitOr(inRepo, "git rev-parse HEAD"),
		"git_commit_sha_short": gitOr(inRepo, "first 7 of git_commit_sha"),
		"git_tag":              tag,
		"git_dirty":            dirty,
		"files_checksum":       cksum,
		"git_repo_url":         pickSource(info.GitRepoURL, ci.repoURL, ciSrc["git_repo_url"], "git remote origin (ssh->https)", "none (no origin remote)"),
		"git_repository":       pickSource(info.GitRepository, ci.repository, ciSrc["git_repository"], "git remote origin", "none (no origin remote)"),
		"actor":                pickSource(info.Actor, ci.actor, ciSrc["actor"], "OS user", "none"),
		"event":                eventSource(ci.event, ciSrc["event"], ci.platform != ""),
		"ci_platform":          platform,
		"ci_build_url":         ciOr(info.CIBuildURL, ciSrc["ci_build_url"]),
		"ci_build_number":      ciOr(info.CIBuildNumber, ciSrc["ci_build_number"]),
		"ci_workflow":          ciOr(info.CIWorkflow, ciSrc["ci_workflow"]),
		"runtime_hostname":     "os.Hostname()",
	}
}

// gitOr returns label when in a git repository, else a "not a repository" note.
func gitOr(inRepo bool, label string) string {
	if !inRepo {
		return "none (not a git repository)"
	}
	return label
}

// ciOr returns the CI label when value is set, else "not in CI".
func ciOr(value, label string) string {
	if value == "" {
		return "not in CI"
	}
	return label
}

// eventSource notes the CI source of event, the local default, or — when in CI
// with no recognizable trigger — that nothing matched.
func eventSource(ciEvent, ciLabel string, inCI bool) string {
	switch {
	case ciEvent != "":
		return ciLabel
	case inCI:
		return "default (no recognizable trigger)"
	default:
		return "default (not in CI)"
	}
}

// pickSource describes a value resolved CI-first then local: the CI label when
// the CI value won, the local label when it fell back, the empty label otherwise.
func pickSource(value, ciValue, ciLabel, localLabel, emptyLabel string) string {
	switch {
	case value == "":
		return emptyLabel
	case ciValue != "":
		return ciLabel
	default:
		return localLabel
	}
}
