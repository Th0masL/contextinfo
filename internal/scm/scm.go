// Package scm parses source-control remote URLs (ssh, scp-style, or https) into
// their HTTPS web URL and "owner/repo" slug, and strips embedded credentials. It
// is used by the git and ci detectors and the core package; it has no
// dependencies beyond the standard library.
package scm

import (
	"net/url"
	"strings"
)

// Sanitize strips embedded credentials from an http(s) remote URL. CI checkouts
// often rewrite origin to include a token (e.g. GitLab's
// "https://gitlab-ci-token:<token>@gitlab.com/..."), which must never be reported
// — it would leak into output, tfvars, or Terraform state. SSH and scp-style
// remotes (git@host:path) carry no secret and are left untouched.
func Sanitize(raw string) string {
	if !strings.Contains(raw, "://") {
		return raw // scp-like (git@host:path): the user is an SSH login, not a secret
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	switch u.Scheme {
	case "http", "https":
		u.User = nil // tokens/passwords live in the userinfo of CI checkout URLs
		return u.String()
	default:
		return raw // ssh://, git://: no password in the URL
	}
}

// HTTPSURL converts a git remote URL to its HTTPS web URL (e.g.
// git@github.com:owner/repo.git -> https://github.com/owner/repo), or "" when it
// can't be derived. Best-effort: an SSH host alias from ~/.ssh/config won't
// resolve to the real web host.
func HTTPSURL(remote string) string {
	host, path := hostPath(remote)
	if host == "" || path == "" {
		return ""
	}
	return "https://" + host + "/" + path
}

// Slug returns the "owner/repo" path of a remote URL, or "".
func Slug(remote string) string {
	_, path := hostPath(remote)
	return path
}

// hostPath splits a git remote URL into its host and "owner/repo" path, handling
// scp-style (git@host:owner/repo.git), ssh://, and https:// forms. It returns
// empty strings when the URL can't be parsed.
func hostPath(remote string) (host, path string) {
	switch {
	case remote == "":
		return "", ""
	case strings.Contains(remote, "://"):
		u, err := url.Parse(remote)
		if err != nil {
			return "", ""
		}
		host, path = u.Host, strings.TrimPrefix(u.Path, "/")
	case strings.Contains(remote, "@") && strings.Contains(remote, ":"):
		rest := remote[strings.LastIndex(remote, "@")+1:] // host:owner/repo.git
		colon := strings.Index(rest, ":")
		host, path = rest[:colon], rest[colon+1:]
	default:
		return "", ""
	}
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i] // drop any :port
	}
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, ".git")
	if host == "" || path == "" {
		return "", ""
	}
	return host, path
}
