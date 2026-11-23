package repo

import (
	"context"
	"strings"

	"agentflow/internal/gitwork"
)

// GitHostCredentials keeps tokens issued by different platforms separate.
// An unconfigured provider must not receive the user's GitHub credential.
func (r *UsersRepo) GitHostCredentials(id int64) map[string]string {
	user, _ := r.User(id)
	tokens := map[string]string{
		"gitee.com":  strings.TrimSpace(user.GiteeToken),
		"gitlab.com": strings.TrimSpace(user.GitLabToken),
	}
	for host, token := range tokens {
		if token != "" {
			tokens[host] = "oauth2:" + token
		}
	}
	tokens["github.com"] = formatUserGitCredential(user)
	return tokens
}

func (r *UsersRepo) GitCredentialForRemote(ctx context.Context, id int64, remote string) string {
	ctx = gitwork.WithHostTokens(gitwork.WithToken(ctx, r.GitCredential(ctx, id)), r.GitHostCredentials(id))
	return gitwork.TokenForRemote(ctx, remote)
}
