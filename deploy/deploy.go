// Package deploy is the deploy-rule engine — the condition model and matcher —
// used by the contextinfo core (which applies rules to a detected context) and
// the config subpackage (which builds rules from YAML).
//
// It is public so consumers can also build rules programmatically — e.g. a
// Terraform provider that decodes deploy rules from HCL and feeds them to
// contextinfo.Resolve / contextinfo.WithDeployRules — not just load them from a
// .contextinfo.yaml. Build patterns with GlobPattern/RegexPattern and populate
// Rules/Rule/Cond/FieldMatch directly.
//
// It is decoupled from the core Info via a Lookup function, so it has no
// dependency on the contextinfo package (avoiding an import cycle) and uses only
// the standard library.
package deploy

import (
	"fmt"
	"regexp"
	"strings"
)

// Rules is an ordered rule set plus a default. Resolve starts from Default, then
// overlays the first rule whose condition matches (earlier rules win).
type Rules struct {
	Rules   []Rule
	Default map[string]string
}

// Rule is one rule: a condition and the variables to set when it matches.
type Rule struct {
	If  Cond
	Set map[string]string
}

// Cond is a boolean condition. Its parts are AND-ed: every Field must match,
// every All sub-condition must hold, the Any list must have at least one match,
// and Not (if set) must not hold. A zero Cond matches everything. Nesting
// All/Any/Not gives full &&/||/!/grouping without an expression parser.
type Cond struct {
	Fields []FieldMatch // implicit AND across distinct fields (a plain YAML map)
	All    []Cond       // AND of sub-conditions
	Any    []Cond       // OR of sub-conditions (matches if any holds)
	Not    *Cond        // negation
}

// FieldMatch matches one field (by name, resolved via Lookup) against one or more
// patterns; it holds if any pattern matches (OR) — how a list value like
// event: [push, manual] is expressed.
type FieldMatch struct {
	Field    string
	Patterns []Pattern
}

// Pattern is a compiled matcher for one field value — a glob (translated to an
// anchored regexp) or a user regexp. raw keeps the original text.
type Pattern struct {
	raw string
	re  *regexp.Regexp
}

// Lookup resolves a match-field name to its value; ok is false for an unknown
// field. The core supplies this, bound to a detected Info.
type Lookup func(field string) (value string, ok bool)

// GlobPattern compiles a shell-style glob into a Pattern. `*` matches any run of
// characters (including `/`), `?` matches one; everything else is literal. The
// pattern is anchored, so `main` matches only "main" and `release/*` matches
// "release/anything".
func GlobPattern(glob string) (Pattern, error) {
	var b strings.Builder
	b.WriteByte('^')
	for _, r := range glob {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteByte('$')
	re, err := regexp.Compile(b.String())
	if err != nil {
		return Pattern{}, err
	}
	return Pattern{raw: glob, re: re}, nil
}

// RegexPattern compiles a user-supplied regexp into a Pattern, used as written
// (anchor it yourself with ^…$ for an exact match, as for strict semver).
func RegexPattern(expr string) (Pattern, error) {
	re, err := regexp.Compile(expr)
	if err != nil {
		return Pattern{}, err
	}
	return Pattern{raw: expr, re: re}, nil
}

func (p Pattern) match(s string) bool { return p.re != nil && p.re.MatchString(s) }

// match reports whether any of the field's patterns match its looked-up value.
// An unknown field never matches (the config parser rejects those up front).
func (f FieldMatch) match(lookup Lookup) bool {
	v, ok := lookup(f.Field)
	if !ok {
		return false
	}
	for _, p := range f.Patterns {
		if p.match(v) {
			return true
		}
	}
	return false
}

// match evaluates the boolean tree.
func (c Cond) match(lookup Lookup) bool {
	for _, f := range c.Fields {
		if !f.match(lookup) {
			return false
		}
	}
	for _, sub := range c.All {
		if !sub.match(lookup) {
			return false
		}
	}
	if len(c.Any) > 0 {
		ok := false
		for _, sub := range c.Any {
			if sub.match(lookup) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if c.Not != nil && c.Not.match(lookup) {
		return false
	}
	return true
}

// Resolve returns the variables (Default overlaid by the first matching rule) and
// a parallel map noting where each came from ("deploy: default" / "deploy: rule
// #N matched"). Both maps are non-nil.
func (r Rules) Resolve(lookup Lookup) (vars, src map[string]string) {
	vars = make(map[string]string, len(r.Default))
	src = make(map[string]string, len(r.Default))
	for k, v := range r.Default {
		vars[k] = v
		src[k] = "deploy: default"
	}
	for idx, rule := range r.Rules {
		if rule.If.match(lookup) {
			for k, v := range rule.Set {
				vars[k] = v
				src[k] = fmt.Sprintf("deploy: rule #%d matched", idx+1)
			}
			break // first match wins
		}
	}
	return vars, src
}
