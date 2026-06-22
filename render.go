package contextinfo

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// RenderOptions controls how an Info is rendered. Both knobs are render-time (a
// detected Info can be rendered any number of ways): Prefix is prepended to every
// variable name (envvar/json/tfvars; ignored by Text), and Explain adds a
// "<field>_explained" companion after each field naming where its value came from
// (provenance is always captured during detection; this only gates emission).
type RenderOptions struct {
	Prefix  string
	Explain bool
}

// flatPair is a single flattened key/value pair (value is a string or bool).
type flatPair struct {
	key string
	val any
}

// flatten returns the context as ordered key/value pairs (matching the Info
// field order), applying opts.Prefix and, when opts.Explain is set, interleaving
// each field's "<key>_explained" companion.
func (i Info) flatten(opts RenderOptions) []flatPair {
	prefix := opts.Prefix
	base := []flatPair{
		{prefix + "git_branch", i.GitBranch},
		{prefix + "git_commit_sha", i.GitCommitSHA},
		{prefix + "git_commit_sha_short", i.GitCommitSHAShort},
		{prefix + "git_tag", i.GitTag},
		{prefix + "git_dirty", i.GitDirty},
		{prefix + "files_checksum", i.FilesChecksum},
		{prefix + "git_repo_url", i.GitRepoURL},
		{prefix + "git_repository", i.GitRepository},
		{prefix + "actor", i.Actor},
		{prefix + "event", i.Event},
		{prefix + "ci_platform", i.CIPlatform},
		{prefix + "ci_build_url", i.CIBuildURL},
		{prefix + "ci_build_number", i.CIBuildNumber},
		{prefix + "ci_workflow", i.CIWorkflow},
		{prefix + "runtime_hostname", i.RuntimeHostname},
	}
	// Derived deploy variables (env_name, build_type, …) follow the detected
	// fields, sorted for stable output. A derived key that collides with a
	// built-in field name is skipped so the output stays unique (built-ins win).
	if len(i.derived) > 0 {
		builtin := make(map[string]bool, len(base))
		for _, p := range base {
			builtin[strings.TrimPrefix(p.key, prefix)] = true
		}
		keys := make([]string, 0, len(i.derived))
		for k := range i.derived {
			if !builtin[k] {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			base = append(base, flatPair{prefix + k, i.derived[k]})
		}
	}
	if !opts.Explain {
		return base
	}
	out := make([]flatPair, 0, len(base)*2)
	for _, p := range base {
		field := strings.TrimPrefix(p.key, prefix)
		out = append(out, p, flatPair{p.key + "_explained", i.explained[field]})
	}
	return out
}

// FlatJSON renders the context as a flat JSON object keyed by the Info json tags
// (e.g. "git_commit_sha"), value types preserved (bools stay bool), keys in a
// stable order, rendered per opts (Prefix, Explain).
func (i Info) FlatJSON(opts RenderOptions) ([]byte, error) {
	pairs := i.flatten(opts)
	var b strings.Builder
	b.WriteString("{\n")
	for idx, p := range pairs {
		key, err := json.Marshal(p.key)
		if err != nil {
			return nil, err
		}
		val, err := json.Marshal(p.val)
		if err != nil {
			return nil, err
		}
		sep := ","
		if idx == len(pairs)-1 {
			sep = ""
		}
		fmt.Fprintf(&b, "  %s: %s%s\n", key, val, sep)
	}
	b.WriteString("}\n")
	return []byte(b.String()), nil
}

// TFVarsHCL renders flat Terraform variables as a .tfvars (HCL) document. String
// values are safely quoted, including escaping of the ${ and %{ interpolation
// markers. Rendered per opts (Prefix, Explain).
func (i Info) TFVarsHCL(opts RenderOptions) string {
	pairs := i.flatten(opts)
	width := 0
	for _, p := range pairs {
		if len(p.key) > width {
			width = len(p.key)
		}
	}
	var b strings.Builder
	for _, p := range pairs {
		fmt.Fprintf(&b, "%-*s = %s\n", width, p.key, hclValue(p.val))
	}
	return b.String()
}

// EnvVars renders the context as shell `NAME=value` lines (one per line),
// suitable for sourcing into an environment. String values are single-quoted for
// shell safety (so spaces, URLs, `$`, etc. can't break or inject); booleans are
// bare `true`/`false`. Each name is prefixed with prefix (use "" for none).
//
// To export them for a child process such as terraform:
//
//	set -a; eval "$(contextinfo --format=envvar --prefix TF_VAR_)"; set +a
func (i Info) EnvVars(opts RenderOptions) string {
	var b strings.Builder
	for _, p := range i.flatten(opts) {
		if v, ok := p.val.(bool); ok {
			fmt.Fprintf(&b, "%s=%t\n", p.key, v)
			continue
		}
		fmt.Fprintf(&b, "%s=%s\n", p.key, shellSingleQuote(fmt.Sprint(p.val)))
	}
	return b.String()
}

// Text renders the context as aligned "key  value" lines, one field per line
// (including the "<key>_explained" companions when opts.Explain is set). Text is
// the human-readable format and never prefixes, so opts.Prefix is ignored.
func (i Info) Text(opts RenderOptions) string {
	pairs := i.flatten(RenderOptions{Explain: opts.Explain})
	width := 0
	for _, p := range pairs {
		if len(p.key) > width {
			width = len(p.key)
		}
	}
	var b strings.Builder
	for _, p := range pairs {
		fmt.Fprintf(&b, "%-*s  %s\n", width, p.key, fmt.Sprint(p.val))
	}
	return b.String()
}

// shellSingleQuote returns s as one safe shell word: wrapped in single quotes,
// with any embedded single quote escaped via the close/escape/reopen idiom.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// hclValue renders a flattened value as an HCL literal: a bare true/false for
// booleans, and a quoted, escaped string for everything else.
func hclValue(v any) string {
	switch x := v.(type) {
	case bool:
		if x {
			return "true"
		}
		return "false"
	case string:
		return hclString(x)
	default:
		return hclString(fmt.Sprint(v))
	}
}

// hclString renders s as an HCL quoted string literal, escaping the characters
// that are special inside one — notably the ${ and %{ interpolation/directive
// markers, which must become $${ and %%{ to be treated literally.
func hclString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '$':
			if i+1 < len(s) && s[i+1] == '{' {
				b.WriteString("$${")
				i++
			} else {
				b.WriteByte('$')
			}
		case '%':
			if i+1 < len(s) && s[i+1] == '{' {
				b.WriteString("%%{")
				i++
			} else {
				b.WriteByte('%')
			}
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}
