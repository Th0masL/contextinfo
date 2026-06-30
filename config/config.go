// Package config loads contextinfo's settings from .contextinfo.yaml files
// discovered on disk and merged closest-wins, so the same knobs the CLI flags
// control can also come from a file.
//
// It is a separate package on purpose: the YAML dependency lives here, leaving
// the core contextinfo package (Detect/Info) import-dependency-free. Importers
// that only detect never compile YAML; importers that want file-based config opt
// in by importing this package.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Th0masL/contextinfo"
	"gopkg.in/yaml.v3"
)

// baseName is the per-directory & $HOME config base name; the canonical ".yaml"
// (or fallback ".yml") extension is appended. The system file under /etc is
// non-hidden by convention ("contextinfo.yaml", see Load).
const baseName = ".contextinfo"

// Config mirrors the CLI knobs. The scalar fields are pointers so an unset key
// (nil) is distinguishable from a value set to its zero — which is what makes
// closest-wins merging, and "explicit flags override the file", work correctly.
type Config struct {
	Format        *string       `yaml:"format"`
	Prefix        *string       `yaml:"prefix"`
	FilesChecksum *bool         `yaml:"files_checksum"`
	Explain       *bool         `yaml:"explain"`
	Deploy        *deployConfig `yaml:"deploy"` // parsed deploy rules (see deploy.go)
}

// fileConfig is the on-disk shape of a .contextinfo.yaml: the public settings
// (inlined) plus the config_cascade discovery directive. The directive is consumed
// during Load to bound the cascade and never surfaces on the merged Config, which
// is why it lives here rather than on Config.
type fileConfig struct {
	Config  `yaml:",inline"`
	Cascade *bool `yaml:"config_cascade"` // false = make this file the top of the cascade
}

// LoadOption configures Load, using the same functional-options pattern as
// contextinfo.Option: each option is a closure that sets a field of the private
// loadOptions, passed variadically (e.g. Load(dir, NoCascade())). This keeps
// Load's signature stable as options are added.
type LoadOption func(*loadOptions)

type loadOptions struct {
	noCascade bool
}

// NoCascade makes Load read only the single closest .contextinfo.yaml (the file
// nearest the directory) and ignore all others — no cascading up the tree, no
// $HOME, no /etc. The CLI exposes this as --no-config-cascade. (It differs from a
// file's `config_cascade: false`, which still merges the files between the
// boundary and the directory; NoCascade reads exactly one file.)
func NoCascade() LoadOption {
	return func(o *loadOptions) { o.noCascade = true }
}

// Load discovers and merges config for the given directory: dir and its parents
// up to the git repo root (the directory containing .git), then $HOME, then /etc
// — merged so the file closest to dir wins. Per-directory and $HOME files are
// ".contextinfo.yaml" (or ".contextinfo.yml"); the system file is
// "/etc/contextinfo.yaml" (or ".yml"). The canonical ".yaml" wins when both
// extensions exist in a directory. It returns the merged config and the paths
// actually loaded (lowest- to highest-precedence). Missing files are not an
// error; a malformed file is.
//
// Two ways to limit the cascade: a file may set `config_cascade: false` (discovery
// stops at that file — nothing farther from dir is read), or the caller may pass
// NoCascade() to read only the single closest file.
func Load(dir string, opts ...LoadOption) (Config, []string, error) {
	var lo loadOptions
	for _, o := range opts {
		o(&lo)
	}
	home, _ := os.UserHomeDir()
	return discover(dir, home, "/etc", lo.noCascade)
}

// load is the testable core for the default (full-cascade) behavior, with the
// $HOME and system directories injected so tests can point them at temp dirs.
func load(dir, home, etc string) (Config, []string, error) {
	return discover(dir, home, etc, false)
}

// discover walks the candidate locations closest-first, collecting existing files
// until it hits a `config_cascade: false` boundary (or, when noCascade is set,
// after the first file), then merges them closest-wins.
func discover(dir, home, etc string, noCascade bool) (Config, []string, error) {
	// Candidate base paths, CLOSEST-first: dir → parents → repo root, then $HOME,
	// then /etc. (treeBases returns root→…→dir, so reverse it.)
	var bases []string
	tree := treeBases(dir)
	for i := len(tree) - 1; i >= 0; i-- {
		bases = append(bases, tree[i])
	}
	if home != "" {
		bases = append(bases, filepath.Join(home, baseName))
	}
	if etc != "" {
		bases = append(bases, filepath.Join(etc, "contextinfo"))
	}

	var found []Config
	var loaded []string
	seen := map[string]bool{}
	for _, base := range bases {
		p := pickFile(base)
		if p == "" {
			continue // no file at this location
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if seen[abs] {
			continue // e.g. cwd == $HOME: don't apply the same file twice
		}
		seen[abs] = true

		data, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue // raced away after discovery: treat as absent
			}
			// Present but unreadable (permissions, EISDIR, I/O): the user placed a
			// config here and it silently failing to apply is worse than surfacing
			// it — same visibility as a malformed file.
			return Config{}, nil, fmt.Errorf("reading %s: %w", p, err)
		}
		var fc fileConfig
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return Config{}, nil, fmt.Errorf("parsing %s: %w", p, err)
		}
		found = append(found, fc.Config)
		loaded = append(loaded, p)

		if noCascade {
			break // read only the closest file
		}
		if fc.Cascade != nil && !*fc.Cascade {
			break // boundary: read nothing farther from dir than this file
		}
	}

	// Merge closest-wins: apply most-general first, the closest last. found is
	// closest-first, so iterate it in reverse.
	var merged Config
	for i := len(found) - 1; i >= 0; i-- {
		merged.merge(found[i])
	}
	// Return loaded lowest- to highest-precedence (loaded is closest-first).
	for i, j := 0, len(loaded)-1; i < j; i, j = i+1, j-1 {
		loaded[i], loaded[j] = loaded[j], loaded[i]
	}
	return merged, loaded, nil
}

// pickFile returns base+".yaml" if it exists, else base+".yml", else "". The
// canonical ".yaml" extension wins when both are present.
func pickFile(base string) string {
	for _, ext := range []string{".yaml", ".yml"} {
		p := base + ext
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

// treeBases returns the config base paths (no extension) from the repo root down
// to dir (dir last, highest precedence). If dir is not inside a git repo, only
// dir's base is considered — the upward walk is bounded by the repo, never the
// whole filesystem.
func treeBases(dir string) []string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}
	root := repoRoot(abs) // "" when not in a repo

	var dirs []string
	for d := abs; ; {
		dirs = append(dirs, d)
		if root == "" || d == root {
			break
		}
		parent := filepath.Dir(d)
		if parent == d {
			break // filesystem root reached
		}
		d = parent
	}

	// dirs is [dir, …, root]; reverse so the root is applied first, dir last.
	bases := make([]string, 0, len(dirs))
	for i := len(dirs) - 1; i >= 0; i-- {
		bases = append(bases, filepath.Join(dirs[i], baseName))
	}
	return bases
}

// repoRoot walks up from dir and returns the first directory containing a .git
// entry, or "" if none is found before the filesystem root.
func repoRoot(dir string) string {
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// merge overlays the set (non-nil) fields of o onto c.
func (c *Config) merge(o Config) {
	if o.Format != nil {
		c.Format = o.Format
	}
	if o.Prefix != nil {
		c.Prefix = o.Prefix
	}
	if o.FilesChecksum != nil {
		c.FilesChecksum = o.FilesChecksum
	}
	if o.Explain != nil {
		c.Explain = o.Explain
	}
	if o.Deploy != nil { // closest-wins: a deploy block replaces a less-specific one
		c.Deploy = o.Deploy
	}
}

// DeployRules returns the parsed deploy rules and whether a deploy block was
// configured at all.
func (c Config) DeployRules() (contextinfo.DeployRules, bool) {
	if c.Deploy == nil {
		return contextinfo.DeployRules{}, false
	}
	return c.Deploy.rules, true
}

// DetectOptions converts the detection-affecting settings into contextinfo
// options. Format, Prefix, and Explain drive rendering rather than Detect — read
// Format directly and use RenderOptions for the rest. Pass contextinfo.WithDir(dir)
// separately for the directory being inspected.
func (c Config) DetectOptions() []contextinfo.Option {
	var opts []contextinfo.Option
	if c.FilesChecksum != nil && !*c.FilesChecksum {
		opts = append(opts, contextinfo.WithoutFilesChecksum())
	}
	if c.Deploy != nil {
		opts = append(opts, contextinfo.WithDeployRules(c.Deploy.rules))
	}
	return opts
}

// RenderOptions maps the rendering settings (prefix, explain) to a
// contextinfo.RenderOptions for the EnvVars/FlatJSON/TFVarsHCL/Text methods.
// (Format selects which method to call and is read directly.)
func (c Config) RenderOptions() contextinfo.RenderOptions {
	var ro contextinfo.RenderOptions
	if c.Prefix != nil {
		ro.Prefix = *c.Prefix
	}
	if c.Explain != nil {
		ro.Explain = *c.Explain
	}
	return ro
}
