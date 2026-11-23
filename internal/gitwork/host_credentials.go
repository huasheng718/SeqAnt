package gitwork

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type hostTokensCtxKey struct{}

// WithHostTokens adds host-specific credentials, including empty entries that
// must never fall back to a token issued by another hosting platform.
func WithHostTokens(ctx context.Context, tokens map[string]string) context.Context {
	return context.WithValue(ctx, hostTokensCtxKey{}, tokens)
}

func HostTokensFrom(ctx context.Context) map[string]string {
	if ctx != nil {
		tokens, _ := ctx.Value(hostTokensCtxKey{}).(map[string]string)
		return tokens
	}
	return nil
}

func contextTokens(ctx context.Context) []string {
	tokens := []string{TokenFrom(ctx)}
	for _, token := range HostTokensFrom(ctx) {
		tokens = append(tokens, token)
	}
	sort.Slice(tokens, func(i, j int) bool { return len(tokens[i]) > len(tokens[j]) })
	return tokens
}

func TokenForRemote(ctx context.Context, remote string) string {
	parsed, err := url.Parse(strings.TrimSpace(remote))
	host := ""
	if err == nil {
		host = strings.ToLower(parsed.Hostname())
	}
	if ssh, err := ParseSSHRemote(remote); host == "" && err == nil {
		host = ssh.Host
	}
	if token, ok := HostTokensFrom(ctx)[host]; ok {
		return strings.TrimSpace(token)
	}
	return TokenFrom(ctx)
}

// BindHostCredentials appends URL-scoped helpers to a child's git config. The
// helpers contain only environment variable names; secrets never enter files
// or command arguments. Missing credentials explicitly stop fallback helpers.
func BindHostCredentials(environment map[string]string, tokens map[string]string) {
	hosts := make([]string, 0, len(tokens))
	for host := range tokens {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	count, _ := strconv.Atoi(environment["GIT_CONFIG_COUNT"])
	for i, host := range hosts {
		user, token := SplitToken(tokens[host])
		userEnv := fmt.Sprintf("AGENTFLOW_GIT_HOST_USER_%d", i)
		passEnv := fmt.Sprintf("AGENTFLOW_GIT_HOST_PASSWORD_%d", i)
		environment[userEnv], environment[passEnv] = user, token
		helper := `!f() { test "$1" = get || exit 0; if test -z "$` + passEnv + `"; then printf 'quit=true\n'; else printf 'username=%s\npassword=%s\n' "$` + userEnv + `" "$` + passEnv + `"; fi; }; f`
		for _, scheme := range []string{"https", "http"} {
			for _, value := range []string{"", helper} {
				environment[fmt.Sprintf("GIT_CONFIG_KEY_%d", count)] = "credential." + scheme + "://" + host + ".helper"
				environment[fmt.Sprintf("GIT_CONFIG_VALUE_%d", count)] = value
				count++
			}
		}
	}
	if len(hosts) > 0 {
		environment["GIT_CONFIG_COUNT"] = strconv.Itoa(count)
	}
}
