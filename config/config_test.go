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
