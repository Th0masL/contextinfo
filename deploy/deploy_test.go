package deploy

import "testing"

func mustGlob(t *testing.T, g string) Pattern {
	t.Helper()
	p, err := GlobPattern(g)
	if err != nil {
		t.Fatalf("glob %q: %v", g, err)
	}
	return p
}

func mustRegex(t *testing.T, e string) Pattern {
	t.Helper()
	p, err := RegexPattern(e)
	if err != nil {
		t.Fatalf("regex %q: %v", e, err)
	}
	return p
}

// lk builds a Lookup from a map; a missing key reports the field as unknown.
func lk(m map[string]string) Lookup {
	return func(f string) (string, bool) { v, ok := m[f]; return v, ok }
}

// field is a single-field, single-pattern condition.
func field(name string, p Pattern) Cond {
	return Cond{Fields: []FieldMatch{{Field: name, Patterns: []Pattern{p}}}}
}

func TestGlobPattern(t *testing.T) {
	cases := []struct {
		glob, in string
		want     bool
	}{
		{"main", "main", true},
		{"main", "maintenance", false}, // anchored, not a prefix match
		{"release/*", "release/2.0", true},
		{"release/*", "release/a/b", true}, // * spans '/'
		{"release/*", "main", false},
		{"v*", "v1.2.3", true},
		{"feature/?", "feature/a", true},
		{"feature/?", "feature/ab", false},
	}
	for _, c := range cases {
		if got := mustGlob(t, c.glob).match(c.in); got != c.want {
			t.Errorf("glob %q vs %q = %v, want %v", c.glob, c.in, got, c.want)
		}
	}
}

func TestCondMatch(t *testing.T) {
	main := lk(map[string]string{"branch": "main", "event": "push"})
	pr := lk(map[string]string{"branch": "feature/x", "event": "pull_request"})
	rel := lk(map[string]string{"branch": "release/2.0", "event": "push"})

	branchMain := field("branch", mustGlob(t, "main"))
	releaseGlob := field("branch", mustGlob(t, "release/*"))

	if !branchMain.match(main) || branchMain.match(pr) {
		t.Error("branch:main matched the wrong contexts")
	}

	// (branch main OR release/*) AND NOT pull_request
	notPR := Cond{Not: &Cond{Fields: []FieldMatch{{Field: "event", Patterns: []Pattern{mustGlob(t, "pull_request")}}}}}
	complex := Cond{All: []Cond{{Any: []Cond{branchMain, releaseGlob}}, notPR}}
	if !complex.match(main) || !complex.match(rel) {
		t.Error("complex should match push to main and release/2.0")
	}
	if complex.match(pr) {
		t.Error("complex should not match a PR")
	}

	// A list value (multiple patterns) is an OR over that field.
	pushOrManual := Cond{Fields: []FieldMatch{{Field: "event", Patterns: []Pattern{mustGlob(t, "push"), mustGlob(t, "manual")}}}}
	if !pushOrManual.match(main) || !pushOrManual.match(lk(map[string]string{"event": "manual"})) || pushOrManual.match(pr) {
		t.Error("event:[push,manual] matched the wrong contexts")
	}

	// An empty condition matches everything; an unknown field never matches.
	if !(Cond{}).match(pr) {
		t.Error("empty cond should match anything")
	}
	if field("nope", mustGlob(t, "*")).match(main) {
		t.Error("an unknown field must never match")
	}
}

func TestResolve(t *testing.T) {
	rules := Rules{
		Rules: []Rule{
			{If: field("tag", mustRegex(t, `^v[0-9]+\.[0-9]+\.[0-9]+$`)), Set: map[string]string{"env_name": "prod"}},
			{If: field("branch", mustGlob(t, "main")), Set: map[string]string{"env_name": "prod"}},
		},
		Default: map[string]string{"env_name": "dev"},
	}
	cases := []struct {
		name   string
		lookup Lookup
		want   string
	}{
		{"strict semver tag", lk(map[string]string{"tag": "v1.2.0"}), "prod"},
		{"non-strict tag -> default", lk(map[string]string{"tag": "v1.2"}), "dev"},
		{"main -> rule 2", lk(map[string]string{"branch": "main"}), "prod"},
		{"no match -> default", lk(map[string]string{"branch": "feature/x"}), "dev"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			vars, src := rules.Resolve(c.lookup)
			if vars["env_name"] != c.want {
				t.Errorf("env_name = %q, want %q", vars["env_name"], c.want)
			}
			if src["env_name"] == "" {
				t.Error("expected a provenance note for env_name")
			}
		})
	}
}
