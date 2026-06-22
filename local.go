package contextinfo

import (
	"os"
	"os/user"
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

// firstNonEmpty returns the first non-empty argument, or "".
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
