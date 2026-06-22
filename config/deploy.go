package config

import (
	"fmt"

	"github.com/Th0masL/contextinfo"
	"github.com/Th0masL/contextinfo/deploy"
	"gopkg.in/yaml.v3"
)

// This file parses the `deploy:` block of a .contextinfo.yaml into the internal
// deploy-rule types. It works on yaml.Node directly (rather than
// struct tags) for two reasons: the condition syntax is polymorphic (a field's
// value may be a scalar, a list, or a {regex: …}/{glob: …} map, and a condition
// may be a field map or an all/any/not combinator), and reading node.Value gives
// the verbatim scalar text — so a value like `event: on` or `tag: 1.0` is matched
// as the string the user wrote, sidestepping YAML's bool/number coercion.

// deployConfig wraps the parsed rules so it can implement yaml.Unmarshaler while
// living in a *Config field.
type deployConfig struct {
	rules deploy.Rules
}

// UnmarshalYAML parses the deploy block when a Config is decoded.
func (d *deployConfig) UnmarshalYAML(node *yaml.Node) error {
	rules, err := parseDeploy(node)
	if err != nil {
		return err
	}
	d.rules = rules
	return nil
}

// parseDeploy reads the deploy mapping: a `rules:` sequence and an optional
// `default:` block.
func parseDeploy(node *yaml.Node) (deploy.Rules, error) {
	node = resolve(node)
	var out deploy.Rules
	if node.Kind != yaml.MappingNode {
		return out, fmt.Errorf("deploy: expected a mapping, got %s", kind(node))
	}
	for k, v := range mapEntries(node) {
		switch k {
		case "rules":
			v = resolve(v)
			if v.Kind != yaml.SequenceNode {
				return out, fmt.Errorf("deploy.rules: expected a list, got %s", kind(v))
			}
			for i, item := range v.Content {
				rule, err := parseRule(item)
				if err != nil {
					return out, fmt.Errorf("deploy.rules[%d]: %w", i, err)
				}
				out.Rules = append(out.Rules, rule)
			}
		case "default":
			def, err := parseRule(v) // default is a rule without (or ignoring) `if`
			if err != nil {
				return out, fmt.Errorf("deploy.default: %w", err)
			}
			out.Default = def.Set
		default:
			return out, fmt.Errorf("deploy: unknown key %q (want rules, default)", k)
		}
	}
	return out, nil
}

// parseRule reads a rule mapping: an optional `if:` condition and a `set:` map.
func parseRule(node *yaml.Node) (deploy.Rule, error) {
	node = resolve(node)
	var rule deploy.Rule
	if node.Kind != yaml.MappingNode {
		return rule, fmt.Errorf("expected a mapping, got %s", kind(node))
	}
	for k, v := range mapEntries(node) {
		switch k {
		case "if":
			cond, err := parseCond(v)
			if err != nil {
				return rule, fmt.Errorf("if: %w", err)
			}
			rule.If = cond
		case "set":
			set, err := parseSet(v)
			if err != nil {
				return rule, fmt.Errorf("set: %w", err)
			}
			rule.Set = set
		default:
			return rule, fmt.Errorf("unknown key %q (want if, set)", k)
		}
	}
	return rule, nil
}

// parseSet reads the `set:` mapping of variable name -> value. Values are taken
// verbatim as strings (so numbers/bools written there become their text).
func parseSet(node *yaml.Node) (map[string]string, error) {
	node = resolve(node)
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected a mapping, got %s", kind(node))
	}
	set := map[string]string{}
	for k, v := range mapEntries(node) {
		v = resolve(v)
		if v.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("%s: expected a scalar value, got %s", k, kind(v))
		}
		set[k] = v.Value
	}
	if len(set) == 0 {
		return nil, fmt.Errorf("must set at least one variable")
	}
	return set, nil
}

// parseCond reads a condition mapping. Keys all/any/not are combinators; every
// other key is a field name matched against its value(s).
func parseCond(node *yaml.Node) (deploy.Cond, error) {
	node = resolve(node)
	var cond deploy.Cond
	if node.Kind != yaml.MappingNode {
		return cond, fmt.Errorf("expected a mapping, got %s", kind(node))
	}
	for k, v := range mapEntries(node) {
		switch k {
		case "all", "any":
			subs, err := parseCondList(v)
			if err != nil {
				return cond, fmt.Errorf("%s: %w", k, err)
			}
			if k == "all" {
				cond.All = subs
			} else {
				cond.Any = subs
			}
		case "not":
			sub, err := parseCond(v)
			if err != nil {
				return cond, fmt.Errorf("not: %w", err)
			}
			cond.Not = &sub
		default:
			if !contextinfo.IsMatchField(k) {
				return cond, fmt.Errorf("unknown match field %q", k)
			}
			pats, err := parsePatterns(v)
			if err != nil {
				return cond, fmt.Errorf("%s: %w", k, err)
			}
			cond.Fields = append(cond.Fields, deploy.FieldMatch{Field: k, Patterns: pats})
		}
	}
	return cond, nil
}

// parseCondList reads a sequence of conditions (the value of all:/any:).
func parseCondList(node *yaml.Node) ([]deploy.Cond, error) {
	node = resolve(node)
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("expected a list, got %s", kind(node))
	}
	var out []deploy.Cond
	for i, item := range node.Content {
		c, err := parseCond(item)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		out = append(out, c)
	}
	return out, nil
}

// parsePatterns reads a field's value: a scalar (one glob), a {regex:…}/{glob:…}
// map (one pattern), or a list of either (OR over patterns).
func parsePatterns(node *yaml.Node) ([]deploy.Pattern, error) {
	node = resolve(node)
	if node.Kind == yaml.SequenceNode {
		var pats []deploy.Pattern
		for i, item := range node.Content {
			p, err := parsePattern(item)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			pats = append(pats, p)
		}
		if len(pats) == 0 {
			return nil, fmt.Errorf("empty list")
		}
		return pats, nil
	}
	p, err := parsePattern(node)
	if err != nil {
		return nil, err
	}
	return []deploy.Pattern{p}, nil
}

// parsePattern reads a single pattern: a scalar (glob) or a one-key mapping
// {regex: …} or {glob: …}.
func parsePattern(node *yaml.Node) (deploy.Pattern, error) {
	node = resolve(node)
	switch node.Kind {
	case yaml.ScalarNode:
		return deploy.GlobPattern(node.Value)
	case yaml.MappingNode:
		entries := node.Content
		if len(entries) != 2 { // exactly one key/value pair
			return deploy.Pattern{}, fmt.Errorf("expected a single {regex: …} or {glob: …}")
		}
		key, val := entries[0].Value, resolve(entries[1])
		if val.Kind != yaml.ScalarNode {
			return deploy.Pattern{}, fmt.Errorf("%s: expected a scalar", key)
		}
		switch key {
		case "regex":
			p, err := deploy.RegexPattern(val.Value)
			if err != nil {
				return deploy.Pattern{}, fmt.Errorf("regex %q: %w", val.Value, err)
			}
			return p, nil
		case "glob":
			return deploy.GlobPattern(val.Value)
		default:
			return deploy.Pattern{}, fmt.Errorf("unknown matcher %q (want regex or glob)", key)
		}
	default:
		return deploy.Pattern{}, fmt.Errorf("expected a scalar, list, or {regex:/glob:} map, got %s", kind(node))
	}
}

// mapEntries iterates a mapping node's key/value pairs, yielding the key string
// and the value node.
func mapEntries(node *yaml.Node) map[string]*yaml.Node {
	out := make(map[string]*yaml.Node, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		out[node.Content[i].Value] = node.Content[i+1]
	}
	return out
}

// resolve dereferences an alias node to the node it points at (so YAML anchors
// work); non-alias nodes are returned unchanged.
func resolve(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.AliasNode {
		return node.Alias
	}
	return node
}

// kind renders a node kind for error messages.
func kind(node *yaml.Node) string {
	switch resolve(node).Kind {
	case yaml.ScalarNode:
		return "a scalar"
	case yaml.MappingNode:
		return "a mapping"
	case yaml.SequenceNode:
		return "a list"
	default:
		return "nothing"
	}
}
