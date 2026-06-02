package security

import (
	"net/url"
	"strings"
)

var (
	allowedRedirectHostSuffixes = []string{
		".claude.ai",
		".anthropic.com",
	}
	allowedRedirectHostsExact = map[string]struct{}{
		"claude.ai":     {},
		"anthropic.com": {},
	}
)

func IsAllowedRedirect(uri string) bool {
	u, err := url.Parse(uri)
	if err != nil {
		return false
	}
	if u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}

	if _, ok := allowedRedirectHostsExact[host]; ok {
		return true
	}

	for _, suffix := range allowedRedirectHostSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}

	return false
}
