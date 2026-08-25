package reddit

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,30}$`)

func NormalizeUsername(input string) (string, error) {
	name := strings.TrimSpace(input)
	if parsed, err := url.Parse(name); err == nil && parsed.Host != "" {
		host := strings.ToLower(parsed.Hostname())
		if host != "reddit.com" && !strings.HasSuffix(host, ".reddit.com") {
			return "", fmt.Errorf("expected a reddit.com profile URL")
		}
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) >= 2 && (strings.EqualFold(parts[0], "u") || strings.EqualFold(parts[0], "user")) {
			name = parts[1]
		}
	}
	name = strings.TrimPrefix(name, "@")
	name = strings.TrimPrefix(name, "/")
	if len(name) >= 2 && (strings.EqualFold(name[:2], "u/") || strings.HasPrefix(name, "/u/")) {
		name = strings.TrimPrefix(name, "/")
		name = name[2:]
	}
	name = strings.Trim(name, "/")
	if !usernamePattern.MatchString(name) {
		return "", fmt.Errorf("use 1-30 letters, numbers, underscores, or hyphens")
	}
	return name, nil
}
