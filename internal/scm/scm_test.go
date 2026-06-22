package scm

import (
	"strings"
	"testing"
)

// Credentials embedded in an http(s) remote are stripped; other forms are left
// untouched. The token must never survive into the output.
func TestSanitize(t *testing.T) {
	cases := map[string]string{
		// GitLab CI rewrites origin with the job token — must be stripped.
		"https://gitlab-ci-token:secrettoken@gitlab.com/o/r.git": "https://gitlab.com/o/r.git",
		"https://user:pa55w0rd@example.com/o/r.git":              "https://example.com/o/r.git",
		// No credentials / not http(s): left as-is.
		"https://github.com/o/r.git":   "https://github.com/o/r.git",
		"git@github.com:o/r.git":       "git@github.com:o/r.git",
		"ssh://git@gitlab.com/o/r.git": "ssh://git@gitlab.com/o/r.git",
		"":                             "",
	}
	for in, want := range cases {
		got := Sanitize(in)
		if got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
		if strings.Contains(got, "secrettoken") || strings.Contains(got, "pa55w0rd") {
			t.Errorf("Sanitize(%q) leaked a credential: %q", in, got)
		}
	}
}

// hostPath splits scp-style, ssh://, and https:// remotes into host and
// owner/repo, dropping the .git suffix and any port, and returns empty on junk.
func TestHostPath(t *testing.T) {
	cases := []struct{ in, host, path string }{
		{"git@github.com:acme/widgets.git", "github.com", "acme/widgets"},
		{"https://github.com/acme/widgets.git", "github.com", "acme/widgets"},
		{"https://github.com/acme/widgets", "github.com", "acme/widgets"},
		{"ssh://git@gitlab.com/grp/sub/proj.git", "gitlab.com", "grp/sub/proj"},
		{"ssh://git@example.com:2222/o/r.git", "example.com", "o/r"},
		{"git@gitlab.com:grp/sub/proj.git", "gitlab.com", "grp/sub/proj"},
		{"", "", ""},
		{"not a url", "", ""},
	}
	for _, c := range cases {
		h, p := hostPath(c.in)
		if h != c.host || p != c.path {
			t.Errorf("hostPath(%q) = (%q, %q), want (%q, %q)", c.in, h, p, c.host, c.path)
		}
	}
}

// HTTPSURL builds the web URL and Slug the owner/repo path; neither may leak a
// credential even from an unsanitized token URL.
func TestHTTPSURLAndSlug(t *testing.T) {
	cases := []struct{ remote, url, slug string }{
		{"git@github.com:acme/widgets.git", "https://github.com/acme/widgets", "acme/widgets"},
		// Even an unsanitized token URL must not leak credentials into the web URL.
		{"https://gitlab-ci-token:tok@gitlab.com/o/r.git", "https://gitlab.com/o/r", "o/r"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := HTTPSURL(c.remote); got != c.url {
			t.Errorf("HTTPSURL(%q) = %q, want %q", c.remote, got, c.url)
		}
		if strings.Contains(HTTPSURL(c.remote), "@") {
			t.Errorf("HTTPSURL(%q) leaked a credential", c.remote)
		}
		if got := Slug(c.remote); got != c.slug {
			t.Errorf("Slug(%q) = %q, want %q", c.remote, got, c.slug)
		}
	}
}
