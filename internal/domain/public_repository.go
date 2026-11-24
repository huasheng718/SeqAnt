package domain

import (
	"net/url"
	"regexp"
	"strings"
)

var hostedRepositorySegment = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

var githubRepositoryPath = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// ParseRepositoryURL adds public hosting without changing the upstream adapter.
// The prefixed remote identity prevents collisions with enterprise project IDs.
func ParseRepositoryURL(raw string) (GongfengRepoInfo, bool) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "git clone ")
	s = strings.Trim(s, "\"'")
	for _, provider := range []string{"github", "gitee", "gitlab", "codeup"} {
		if strings.HasPrefix(s, provider+":") {
			host := provider + ".com"
			if provider == "codeup" {
				host = "codeup.aliyun.com"
			}
			s = "https://" + host + "/" + strings.TrimPrefix(s, provider+":")
		}
	}
	for _, prefix := range []string{"git@github.com:", "ssh://git@github.com/", "ssh://git@github.com:22/"} {
		if strings.HasPrefix(s, prefix) {
			s = "https://github.com/" + strings.TrimPrefix(s, prefix)
			break
		}
	}
	if strings.HasPrefix(s, "github.com/") || strings.HasPrefix(s, "gitee.com/") || strings.HasPrefix(s, "gitlab.com/") || strings.HasPrefix(s, "codeup.aliyun.com/") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err == nil && u.Host == "github.com" && u.User == nil && (u.Scheme == "https" || u.Scheme == "http") {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 {
			return GongfengRepoInfo{}, false
		}
		path := parts[0] + "/" + strings.TrimSuffix(parts[1], ".git")
		if !githubRepositoryPath.MatchString(path) || parts[0] == "." || parts[0] == ".." || parts[1] == "." || parts[1] == ".." {
			return GongfengRepoInfo{}, false
		}
		base := "https://github.com/" + path
		return GongfengRepoInfo{Project: "github:" + path, Name: strings.TrimSuffix(parts[1], ".git"), WebURL: base, CloneURL: base + ".git"}, true
	}
	if err == nil && (strings.EqualFold(u.Host, "gitee.com") || strings.EqualFold(u.Host, "gitlab.com")) && u.User == nil && (u.Scheme == "https" || u.Scheme == "http") {
		provider := strings.TrimSuffix(strings.ToLower(u.Host), ".com")
		path := strings.SplitN(strings.Trim(u.Path, "/"), "/-/", 2)[0]
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			return GongfengRepoInfo{}, false
		}
		if provider == "gitee" {
			parts = parts[:2]
		}
		parts[len(parts)-1] = strings.TrimSuffix(parts[len(parts)-1], ".git")
		for _, part := range parts {
			if part == "." || part == ".." || !hostedRepositorySegment.MatchString(part) {
				return GongfengRepoInfo{}, false
			}
		}
		path = strings.Join(parts, "/")
		base := "https://" + strings.ToLower(u.Host) + "/" + path
		return GongfengRepoInfo{Project: provider + ":" + path, Name: parts[len(parts)-1], WebURL: base, CloneURL: base + ".git"}, true
	}
	if err == nil && strings.EqualFold(u.Host, "codeup.aliyun.com") && u.User == nil && (u.Scheme == "https" || u.Scheme == "http") {
		parts := strings.Split(strings.Trim(strings.SplitN(strings.Trim(u.Path, "/"), "/-/", 2)[0], "/"), "/")
		if len(parts) < 2 {
			return GongfengRepoInfo{}, false
		}
		for _, part := range parts {
			if part == "." || part == ".." || !hostedRepositorySegment.MatchString(part) {
				return GongfengRepoInfo{}, false
			}
		}
		parts[len(parts)-1] = strings.TrimSuffix(parts[len(parts)-1], ".git")
		path := strings.Join(parts, "/")
		base := "https://codeup.aliyun.com/" + path
		return GongfengRepoInfo{Project: "codeup:" + path, Name: parts[len(parts)-1], WebURL: base, CloneURL: base + ".git"}, true
	}
	return ParseGongfengRepoURL(raw)
}
