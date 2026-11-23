package gitwork

import (
	"context"
	"strings"
	"testing"
)

func TestHostCredentialsUseRealGitHelpers(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	ctx := WithHostTokens(WithToken(context.Background(), "github-secret"), map[string]string{
		"gitee.com": "gitee-secret-with-'-$()-quotes", "gitlab.com": "gitlab-secret",
	})
	for _, tc := range []struct{ host, token string }{
		{"gitee.com", "gitee-secret-with-'-$()-quotes"},
		{"gitlab.com", "gitlab-secret"},
		{"github.com", "github-secret"},
		{"gitee.com.attacker.invalid", "github-secret"},
	} {
		t.Run(tc.host, func(t *testing.T) {
			cmd := command(ctx, t.TempDir(), TokenFrom(ctx), nil, []string{"credential", "fill"})
			cmd.Stdin = strings.NewReader("protocol=https\nhost=" + tc.host + "\n\n")
			out, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(out), "password="+tc.token+"\n") {
				t.Fatal("git credential fill did not select the expected platform token")
			}
		})
	}
	ctx = WithHostTokens(ctx, map[string]string{"gitee.com": ""})
	cmd := command(ctx, t.TempDir(), TokenFrom(ctx), nil, []string{"credential", "fill"})
	cmd.Stdin = strings.NewReader("protocol=https\nhost=gitee.com\n\n")
	if out, err := cmd.CombinedOutput(); err == nil || strings.Contains(string(out), "github-secret") {
		t.Fatal("missing Gitee token fell back to a different platform")
	}
}

func TestHostTokenSelectionAndRedaction(t *testing.T) {
	ctx := WithHostTokens(WithToken(context.Background(), "github-secret"), map[string]string{"gitee.com": "gitee-secret", "gitlab.com": ""})
	for _, tc := range []struct{ remote, token string }{
		{"https://GITEE.COM/team/repo.git", "gitee-secret"},
		{"https://gitlab.com/team/repo.git", ""},
		{"https://github.com/team/repo.git", "github-secret"},
	} {
		if TokenForRemote(ctx, tc.remote) != tc.token {
			t.Fatal("incorrect token selected for " + tc.remote)
		}
	}
	if got := Redact("github-secret gitee-secret", contextTokens(ctx)...); got != "*** ***" {
		t.Fatal("host credentials were not redacted")
	}
	if TokenFrom(WithToken(ctx, "")) != "" {
		t.Fatal("clearing a selected token must shadow its parent context")
	}
}
