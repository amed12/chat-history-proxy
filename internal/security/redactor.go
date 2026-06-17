package security

import (
	"net/url"
	"regexp"
)

var kvPasswordRegex = regexp.MustCompile(`(?i)(password\s*=\s*)(['"]?)([^'"\s]+)(['"]?)`)

// MaskDatabaseURL replaces the password in a connection string (either URL or key-value format) with "xxx".
func MaskDatabaseURL(dsn string) string {
	if dsn == "" {
		return ""
	}

	// Try URL parsing first
	u, err := url.Parse(dsn)
	if err == nil && u.Scheme != "" {
		if u.User != nil {
			if _, hasPass := u.User.Password(); hasPass {
				u.User = url.UserPassword(u.User.Username(), "xxx")
			}
		}
		return u.String()
	}

	// Key-value fallback (e.g. host=localhost user=postgres password=secret)
	return kvPasswordRegex.ReplaceAllString(dsn, `${1}${2}xxx${4}`)
}
