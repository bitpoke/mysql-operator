package url

import (
	"net/url"
	"strings"
)

// MatchesHost matches an URL against a host taking into consideration common ports (80 for http & 443 for https).
func MatchesHost(u1 url.URL, h2 string, allowSubdomain bool) bool {
	if result := matches(u1.Host, h2, allowSubdomain); result {
		return true
	}
	if u1.Scheme == "http" && strings.HasSuffix(h2, ":80") {
		return matches(u1.Hostname(), h2[:len(h2)-len(":80")], allowSubdomain)
	} else if u1.Scheme == "https" && strings.HasSuffix(h2, ":443") {
		return matches(u1.Hostname(), h2[:len(h2)-len(":443")], allowSubdomain)
	}
	return false
}

func matches(h1, h2 string, allowSubdomain bool) bool {
	return h1 == h2 ||
		(allowSubdomain && strings.HasSuffix(h1, "."+h2))
}
