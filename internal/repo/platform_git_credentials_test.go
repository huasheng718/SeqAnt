package repo

import (
	"context"
	"testing"

	"agentflow/internal/domain"
)

func TestPlatformGitCredentialsRemainSeparate(t *testing.T) {
	st, err := OpenTest()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.PutUser(domain.User{LoginName: "platform-git-owner", GitToken: "github-secret", GiteeToken: "gitee-secret", GitLabToken: "gitlab-secret"}); err != nil {
		t.Fatal(err)
	}
	user, ok := st.UserByLoginName("platform-git-owner")
	if !ok {
		t.Fatal("missing credential owner")
	}
	if err := st.PutUser(domain.User{ID: user.ID, LoginName: user.LoginName}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ remote, token string }{
		{"https://github.com/team/repo.git", "github-secret"},
		{"https://GITEE.COM/team/repo.git", "oauth2:gitee-secret"},
		{"https://gitlab.com/team/repo.git", "oauth2:gitlab-secret"},
		{"git@gitee.com:team/repo.git", "oauth2:gitee-secret"},
	} {
		if st.GitCredentialForRemote(context.Background(), user.ID, tc.remote) != tc.token {
			t.Fatal("incorrect credential for " + tc.remote)
		}
	}
	if _, err := st.UpdateUserCredentials(user.ID, UserCredentialUpdate{GiteeTokenClear: true}); err != nil {
		t.Fatal(err)
	}
	if st.GitCredentialForRemote(context.Background(), user.ID, "https://gitee.com/team/repo.git") != "" {
		t.Fatal("removed Gitee token fell back to GitHub")
	}
	stored, _ := st.User(user.ID)
	public := domain.PublicUser(stored)
	if public.GiteeTokenConfigured || !public.GitLabTokenConfigured || !public.GitTokenConfigured || public.GitLabToken != "" || public.GitToken != "" || stored.GitLabToken != "gitlab-secret" {
		t.Fatal("provider deletion changed another credential or exposed its secret")
	}
}
