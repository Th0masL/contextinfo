package contextinfo

import (
	"net/url"
	"os"
	"os/user"
	"strings"
)

// osUser returns the current local user's name, falling back to the USER /
// USERNAME environment variables, or "".
func osUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return firstNonEmpty(os.Getenv("USER"), os.Getenv("USERNAME"))
}

// hostname returns the host name, or "".
func hostname() string {
	h, _ := os.Hostname()
	return h
}

// sanitizeRemote strips embedded credentials from an http(s) remote URL. CI
// checkouts often rewrite origin to include a token (e.g. GitLab's
// "https://gitlab-ci-token:<token>@gitlab.com/..."), which must never be
// reported — it would leak into output, tfvars, or Terraform state. SSH and
// scp-style remotes (git@host:path) carry no secret and are left untouched.
func sanitizeRemote(raw string) string {
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

// httpsRepoURL converts a git remote URL to its HTTPS web URL (e.g.
// git@github.com:owner/repo.git -> https://github.com/owner/repo), or "" when it
// can't be derived. Best-effort: an SSH host alias from ~/.ssh/config won't
// resolve to the real web host.
func httpsRepoURL(remote string) string {
	host, path := remoteHostPath(remote)
	if host == "" || path == "" {
		return ""
	}
	return "https://" + host + "/" + path
}

// repoSlug returns the "owner/repo" path of a remote URL, or "".
func repoSlug(remote string) string {
	_, path := remoteHostPath(remote)
	return path
}

// remoteHostPath splits a git remote URL into its host and "owner/repo" path,
// handling scp-style (git@host:owner/repo.git), ssh://, and https:// forms. It
// returns empty strings when the URL can't be parsed.
func remoteHostPath(remote string) (host, path string) {
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
