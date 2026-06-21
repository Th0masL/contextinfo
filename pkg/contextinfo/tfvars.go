package contextinfo

import (
	"encoding/json"
	"fmt"
	"strings"
)

// tfvarPrefix namespaces the flattened Terraform variable names.
const tfvarPrefix = "contextinfo_"

// tfvar is a single flattened Terraform variable (value is a string or bool).
type tfvar struct {
	name string
	val  any
}

// tfvars flattens the context into Terraform variables in a stable, grouped
// order (ci, git, runtime), each prefixed with "contextinfo_".
func (i Info) tfvars() []tfvar {
	return []tfvar{
		{tfvarPrefix + "ci_detected", i.CI.Detected},
		{tfvarPrefix + "ci_name", i.CI.Name},
		{tfvarPrefix + "ci_build_url", i.CI.BuildURL},
		{tfvarPrefix + "ci_build_number", i.CI.BuildNumber},
		{tfvarPrefix + "git_commit", i.Git.Commit},
		{tfvarPrefix + "git_branch", i.Git.Branch},
		{tfvarPrefix + "git_tag", i.Git.Tag},
		{tfvarPrefix + "git_dirty", i.Git.Dirty},
		{tfvarPrefix + "git_remote", i.Git.Remote},
		{tfvarPrefix + "runtime_os", i.Runtime.OS},
		{tfvarPrefix + "runtime_arch", i.Runtime.Arch},
		{tfvarPrefix + "runtime_hostname", i.Runtime.Hostname},
	}
}

// TFVarsJSON renders the context as a Terraform variables document in JSON
// (suitable for a *.auto.tfvars.json file).
func (i Info) TFVarsJSON() ([]byte, error) {
	m := make(map[string]any, 12)
	for _, v := range i.tfvars() {
		m[v.name] = v.val
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// TFVarsHCL renders the context as a Terraform variables document in HCL
// (suitable for a *.auto.tfvars file). String values are safely quoted,
// including escaping of the ${ and %{ interpolation markers.
func (i Info) TFVarsHCL() string {
	vars := i.tfvars()
	width := 0
	for _, v := range vars {
		if len(v.name) > width {
			width = len(v.name)
		}
	}
	var b strings.Builder
	for _, v := range vars {
		fmt.Fprintf(&b, "%-*s = %s\n", width, v.name, hclValue(v.val))
	}
	return b.String()
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
