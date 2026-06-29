package config

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Files are merged closest-wins across /etc, $HOME, and the repo tree (repo root
// down to the working dir), with the working dir highest.
func TestLoadMergePrecedence(t *testing.T) {
	etc := t.TempDir()
	home := t.TempDir()
	repo := t.TempDir()
	sub := filepath.Join(repo, "stack")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil { // mark repo root
		t.Fatal(err)
	}

	write(t, filepath.Join(etc, "contextinfo.yaml"), "files_checksum: false\nformat: json\n")
	write(t, filepath.Join(home, ".contextinfo.yaml"), "format: envvar\nexplain: true\n")
	write(t, filepath.Join(repo, ".contextinfo.yaml"), "format: text\n")
	write(t, filepath.Join(sub, ".contextinfo.yaml"), "prefix: P_\n")

	cfg, loaded, err := load(sub, home, etc)
	if err != nil {
		t.Fatal(err)
	}

	if got := deref(cfg.Format); got != "text" { // repo overrides home/etc; sub sets no format
		t.Errorf("format = %q, want text", got)
	}
	if got := deref(cfg.Prefix); got != "P_" { // from the closest file (sub)
		t.Errorf("prefix = %q, want P_", got)
	}
	if cfg.FilesChecksum == nil || *cfg.FilesChecksum { // only /etc sets it (false)
		t.Errorf("files_checksum = %v, want false", cfg.FilesChecksum)
	}
	if cfg.Explain == nil || !*cfg.Explain { // only $HOME sets it (true)
		t.Errorf("explain = %v, want true", cfg.Explain)
	}
	if len(loaded) != 4 {
		t.Errorf("loaded %d files, want 4: %v", len(loaded), loaded)
	}
}

// The upward walk stops at the repo root; a config above it is never read.
func TestLoadStopsAtRepoRoot(t *testing.T) {
	above := t.TempDir()
	repo := filepath.Join(above, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(above, ".contextinfo.yaml"), "format: text\n") // above the repo
	write(t, filepath.Join(repo, ".contextinfo.yaml"), "prefix: R_\n")

	cfg, loaded, err := load(repo, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if deref(cfg.Format) != "" {
		t.Errorf("format = %q, want empty (config above the repo root must not be read)", deref(cfg.Format))
	}
	if deref(cfg.Prefix) != "R_" {
		t.Errorf("prefix = %q, want R_", deref(cfg.Prefix))
	}
	if len(loaded) != 1 {
		t.Errorf("loaded %v, want only the repo-root file", loaded)
	}
}

// Outside a git repo, only the working dir's file is read (no upward walk).
func TestLoadNoRepoOnlyCwd(t *testing.T) {
	parent := t.TempDir()
	cwd := filepath.Join(parent, "child")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(parent, ".contextinfo.yaml"), "format: text\n")
	write(t, filepath.Join(cwd, ".contextinfo.yaml"), "prefix: C_\n")

	cfg, _, err := load(cwd, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if deref(cfg.Format) != "" {
		t.Error("a parent's config must not be read when not in a git repo")
	}
	if deref(cfg.Prefix) != "C_" {
		t.Errorf("prefix = %q, want C_", deref(cfg.Prefix))
	}
}

// Both extensions present in a directory: the canonical .yaml wins; .yml is the
// fallback when .yaml is absent.
func TestLoadYamlPreferredOverYml(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".contextinfo.yaml"), "prefix: FROM_YAML_\n")
	write(t, filepath.Join(dir, ".contextinfo.yml"), "prefix: FROM_YML_\n")
	cfg, _, err := load(dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if deref(cfg.Prefix) != "FROM_YAML_" {
		t.Errorf("prefix = %q, want FROM_YAML_ (.yaml must win over .yml)", deref(cfg.Prefix))
	}

	// With only .yml present, it is used.
	dir2 := t.TempDir()
	write(t, filepath.Join(dir2, ".contextinfo.yml"), "prefix: ONLY_YML_\n")
	cfg2, _, err := load(dir2, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if deref(cfg2.Prefix) != "ONLY_YML_" {
		t.Errorf("prefix = %q, want ONLY_YML_ (.yml fallback)", deref(cfg2.Prefix))
	}
}

// A file with config_cascade:false caps the cascade at that file: closer files
// still merge, but $HOME, /etc, and anything farther are ignored.
func TestLoadConfigCascadeFalseBoundary(t *testing.T) {
	etc := t.TempDir()
	home := t.TempDir()
	repo := t.TempDir()
	sub := filepath.Join(repo, "stack")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(etc, "contextinfo.yaml"), "files_checksum: false\n")
	write(t, filepath.Join(home, ".contextinfo.yaml"), "explain: true\n")
	write(t, filepath.Join(repo, ".contextinfo.yaml"), "config_cascade: false\nformat: text\n") // boundary
	write(t, filepath.Join(sub, ".contextinfo.yaml"), "prefix: P_\n")                           // closest

	cfg, loaded, err := load(sub, home, etc)
	if err != nil {
		t.Fatal(err)
	}
	if deref(cfg.Prefix) != "P_" { // closest file, inside the boundary
		t.Errorf("prefix = %q, want P_", deref(cfg.Prefix))
	}
	if deref(cfg.Format) != "text" { // the boundary file is included
		t.Errorf("format = %q, want text", deref(cfg.Format))
	}
	if cfg.Explain != nil {
		t.Error("explain should be unset: $HOME is beyond the config_cascade:false boundary")
	}
	if cfg.FilesChecksum != nil {
		t.Error("files_checksum should be unset: /etc is beyond the boundary")
	}
	if len(loaded) != 2 {
		t.Errorf("loaded %d files, want 2 (sub + repo): %v", len(loaded), loaded)
	}
}

// When several files set config_cascade:false, the boundary is the one CLOSEST to
// the directory; farther config_cascade:false files are never read.
func TestLoadConfigCascadeFalseClosestWins(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	mid := filepath.Join(repo, "mid")
	leaf := filepath.Join(mid, "leaf")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Two boundaries (repo root and mid); the closer one (mid) must win.
	write(t, filepath.Join(repo, ".contextinfo.yaml"), "config_cascade: false\nformat: text\n")
	write(t, filepath.Join(mid, ".contextinfo.yaml"), "config_cascade: false\nexplain: true\n")
	write(t, filepath.Join(leaf, ".contextinfo.yaml"), "prefix: P_\n")
	write(t, filepath.Join(home, ".contextinfo.yaml"), "files_checksum: false\n")

	cfg, loaded, err := load(leaf, home, "")
	if err != nil {
		t.Fatal(err)
	}
	if deref(cfg.Prefix) != "P_" || cfg.Explain == nil || !*cfg.Explain {
		t.Errorf("expected leaf+mid merged (prefix=P_, explain=true), got %+v", cfg)
	}
	if cfg.Format != nil {
		t.Error("format should be unset: the repo-root boundary is farther than mid's, so never read")
	}
	if len(loaded) != 2 { // leaf + mid only
		t.Errorf("loaded %d files, want 2 (leaf + mid): %v", len(loaded), loaded)
	}
}

// NoCascade reads only the single closest file, ignoring everything else.
func TestLoadNoCascadeClosestOnly(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	sub := filepath.Join(repo, "stack")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(repo, ".contextinfo.yaml"), "format: text\n")
	write(t, filepath.Join(sub, ".contextinfo.yaml"), "prefix: P_\n")

	cfg, loaded, err := discover(sub, home, "", true) // NoCascade
	if err != nil {
		t.Fatal(err)
	}
	if deref(cfg.Prefix) != "P_" {
		t.Errorf("prefix = %q, want P_", deref(cfg.Prefix))
	}
	if cfg.Format != nil {
		t.Error("format should be unset: NoCascade reads only the closest file")
	}
	if len(loaded) != 1 {
		t.Errorf("loaded %d files, want 1 (closest only): %v", len(loaded), loaded)
	}
}

// No files anywhere yields an empty config and no error.
func TestLoadMissingIsEmpty(t *testing.T) {
	cfg, loaded, err := load(t.TempDir(), t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Format != nil || cfg.Prefix != nil || cfg.FilesChecksum != nil || cfg.Explain != nil {
		t.Errorf("expected empty config, got %+v", cfg)
	}
	if len(loaded) != 0 {
		t.Errorf("expected no files loaded, got %v", loaded)
	}
}

// A malformed file is a hard error (unlike a missing one).
func TestLoadMalformed(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".contextinfo.yaml"), "format: [unterminated\n")
	if _, _, err := load(dir, "", ""); err == nil {
		t.Error("expected an error for malformed YAML")
	}
}

// DetectOptions maps only the detection-affecting fields; explain/prefix are
// rendering settings, surfaced via RenderOptions instead.
func TestDetectOptions(t *testing.T) {
	off, on := false, true
	if got := len((Config{FilesChecksum: &off, Explain: &on}).DetectOptions()); got != 1 {
		t.Errorf("DetectOptions len = %d, want 1 (WithoutFilesChecksum only; explain is render-time)", got)
	}
	if got := len((Config{FilesChecksum: &on}).DetectOptions()); got != 0 {
		t.Errorf("DetectOptions len = %d, want 0 (checksum on)", got)
	}
}

// RenderOptions carries the prefix and explain settings to the render methods.
func TestRenderOptions(t *testing.T) {
	pfx, on := "TF_VAR_", true
	ro := (Config{Prefix: &pfx, Explain: &on}).RenderOptions()
	if ro.Prefix != "TF_VAR_" || !ro.Explain {
		t.Errorf("RenderOptions = %+v, want {Prefix:TF_VAR_ Explain:true}", ro)
	}
	if ro := (Config{}).RenderOptions(); ro.Prefix != "" || ro.Explain {
		t.Errorf("empty config RenderOptions = %+v, want zero", ro)
	}
}
