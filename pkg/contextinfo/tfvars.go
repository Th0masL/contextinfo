package contextinfo

import (
	"encoding/json"
	"fmt"
	"strings"
)

// flatPair is a single flattened key/value pair (value is a string or bool).
type flatPair struct {
	key string
	val any
}

// flatten returns the context as ordered key/value pairs, with the nested path
// joined by "_" (ci_name, git_commit, runtime_os, ...) and each key prefixed
// with prefix (use "" for none).
func (i Info) flatten(prefix string) []flatPair {
	return []flatPair{
		{prefix + "ci_detected", i.CI.Detected},
		{prefix + "ci_name", i.CI.Name},
		{prefix + "ci_build_url", i.CI.BuildURL},
		{prefix + "ci_build_number", i.CI.BuildNumber},
		{prefix + "ci_actor", i.CI.Actor},
		{prefix + "ci_event", i.CI.Event},
		{prefix + "ci_repository", i.CI.Repository},
		{prefix + "ci_workflow", i.CI.Workflow},
		{prefix + "ci_server_url", i.CI.ServerURL},
		{prefix + "git_commit", i.Git.Commit},
		{prefix + "git_branch", i.Git.Branch},
		{prefix + "git_tag", i.Git.Tag},
		{prefix + "git_dirty", i.Git.Dirty},
		{prefix + "git_remote", i.Git.Remote},
		{prefix + "runtime_os", i.Runtime.OS},
		{prefix + "runtime_arch", i.Runtime.Arch},
		{prefix + "runtime_hostname", i.Runtime.Hostname},
	}
}

// FlatJSON renders the context as a flat JSON object: nested paths joined with
// "_" (e.g. "git_commit"), value types preserved (bools stay bool), keys in a
// stable order, each prefixed with prefix (use "" for none).
func (i Info) FlatJSON(prefix string) ([]byte, error) {
	pairs := i.flatten(prefix)
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

// TFVarsJSON renders flat Terraform variables as a .tfvars.json document. It is
// equivalent to FlatJSON — a flat, prefixed JSON object.
func (i Info) TFVarsJSON(prefix string) ([]byte, error) {
	return i.FlatJSON(prefix)
}

// TFVarsHCL renders flat Terraform variables as a .tfvars (HCL) document. String
// values are safely quoted, including escaping of the ${ and %{ interpolation
// markers. Each variable name is prefixed with prefix (use "" for none).
func (i Info) TFVarsHCL(prefix string) string {
	pairs := i.flatten(prefix)
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
func (i Info) EnvVars(prefix string) string {
	var b strings.Builder
	for _, p := range i.flatten(prefix) {
		if v, ok := p.val.(bool); ok {
			fmt.Fprintf(&b, "%s=%t\n", p.key, v)
			continue
		}
		fmt.Fprintf(&b, "%s=%s\n", p.key, shellSingleQuote(fmt.Sprint(p.val)))
	}
	return b.String()
}

// shellSingleQuote returns s as one safe shell word: wrapped in single quotes,
// with any embedded single quote escaped via the close/escape/reopen idiom.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

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
