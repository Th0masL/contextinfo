package config

import (
	"testing"

	"github.com/Th0masL/contextinfo"
	"gopkg.in/yaml.v3"
)

// Parse a full deploy block and check it resolves correctly end-to-end —
// exercising regex, all/any/not, list values, and globs.
func TestDeployParseAndResolve(t *testing.T) {
	src := `
deploy:
  rules:
    - if:
        tag:
          regex: '^v[0-9]+\.[0-9]+\.[0-9]+$'
      set: { env_name: prod, build_type: production }
    - if:
        all:
          - any:
              - { branch: main }
              - { branch: master }
          - not: { event: pull_request }
      set: { env_name: prod, build_type: production }
    - if:
        branch: "release/*"
      set: { env_name: dev, build_type: staging }
    - if:
        event: [push, manual]
      set: { env_name: dev, build_type: development }
  default:
    set: { env_name: dev, build_type: development }
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rules, ok := cfg.DeployRules()
	if !ok {
		t.Fatal("expected deploy rules to be present")
	}

	cases := []struct {
		name      string
		info      contextinfo.Info
		env, btyp string
	}{
		{"strict semver tag", contextinfo.Info{GitTag: "v1.2.0"}, "prod", "production"},
		{"non-strict tag falls through", contextinfo.Info{GitTag: "v1.2", Event: "push"}, "dev", "development"},
		{"push to main", contextinfo.Info{GitBranch: "main", Event: "push"}, "prod", "production"},
		{"push to master", contextinfo.Info{GitBranch: "master", Event: "push"}, "prod", "production"},
		{"PR into main excluded", contextinfo.Info{GitBranch: "main", Event: "pull_request"}, "dev", "development"},
		{"release glob", contextinfo.Info{GitBranch: "release/2.0", Event: "push"}, "dev", "staging"},
		{"manual run", contextinfo.Info{Event: "manual"}, "dev", "development"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := contextinfo.Resolve(rules, c.info)
			if got["env_name"] != c.env || got["build_type"] != c.btyp {
				t.Errorf("Resolve(%+v) = %v, want env_name=%s build_type=%s", c.info, got, c.env, c.btyp)
			}
		})
	}
}

// A bare-list value and a glob/regex coexist; verify list + explicit {glob:}.
func TestDeployParseListAndGlobMatcher(t *testing.T) {
	src := `
deploy:
  rules:
    - if:
        branch: [main, "release/*"]
        event: { glob: "push" }
      set: { env_name: prod }
  default:
    set: { env_name: dev }
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rules, _ := cfg.DeployRules()
	if got := contextinfo.Resolve(rules, contextinfo.Info{GitBranch: "release/9", Event: "push"}); got["env_name"] != "prod" {
		t.Errorf("release/9 push -> %v, want prod", got)
	}
	if got := contextinfo.Resolve(rules, contextinfo.Info{GitBranch: "main", Event: "pull_request"}); got["env_name"] != "dev" {
		t.Errorf("main PR -> %v, want dev (event glob push excludes it)", got)
	}
}

// YAML anchors/aliases are supported across the deploy parser (that is the whole
// point of resolve()): a shared set/condition can be defined once and reused.
func TestDeployParseAnchorsAndAliases(t *testing.T) {
	src := `
deploy:
  rules:
    - if: { branch: main }
      set: &prod { env_name: prod, build_type: production }
    - if: { branch: "release/*" }
      set: *prod
  default:
    set: { env_name: dev, build_type: development }
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rules, ok := cfg.DeployRules()
	if !ok {
		t.Fatal("expected deploy rules to be present")
	}
	// The aliased rule must resolve to the same set as the anchor.
	got := contextinfo.Resolve(rules, contextinfo.Info{GitBranch: "release/2.0", Event: "push"})
	if got["env_name"] != "prod" || got["build_type"] != "production" {
		t.Errorf("aliased set -> %v, want env_name=prod build_type=production", got)
	}
}

// A duplicate key inside the (manually-walked) deploy block is rejected, not
// silently last-wins — yaml.Node walking would otherwise lose yaml.v3's own
// duplicate-key safety.
func TestDeployParseRejectsDuplicateKeys(t *testing.T) {
	cases := map[string]string{
		"duplicate field in condition": "deploy:\n  rules:\n    - if: { branch: main, branch: dev }\n      set: { env_name: x }\n",
		"duplicate var in set":         "deploy:\n  rules:\n    - if: { branch: main }\n      set: { env_name: a, env_name: b }\n",
		"duplicate rule key":           "deploy:\n  rules:\n    - if: { branch: main }\n      set: { env_name: x }\n      set: { env_name: y }\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			var cfg Config
			if err := yaml.Unmarshal([]byte(src), &cfg); err == nil {
				t.Errorf("expected a duplicate-key error for %q", name)
			}
		})
	}
}

// Malformed deploy blocks are hard errors with a useful message.
func TestDeployParseErrors(t *testing.T) {
	cases := map[string]string{
		"unknown field":       "deploy:\n  rules:\n    - if: { branche: main }\n      set: { env_name: x }\n",
		"bad regex":           "deploy:\n  rules:\n    - if: { tag: { regex: '([' } }\n      set: { env_name: x }\n",
		"unknown deploy key":  "deploy:\n  ruls: []\n",
		"unknown rule key":    "deploy:\n  rules:\n    - iff: { branch: main }\n",
		"set is not a scalar": "deploy:\n  rules:\n    - if: { branch: main }\n      set: { env_name: [a, b] }\n",
		"empty set":           "deploy:\n  rules:\n    - if: { branch: main }\n      set: {}\n",
		"unknown matcher":     "deploy:\n  rules:\n    - if: { tag: { rex: 'x' } }\n      set: { env_name: x }\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			var cfg Config
			if err := yaml.Unmarshal([]byte(src), &cfg); err == nil {
				t.Errorf("expected an error for %q", name)
			}
		})
	}
}
