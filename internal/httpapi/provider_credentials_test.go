package httpapi

import (
	"agentflow/internal/auth"
	"agentflow/internal/domain"
	"agentflow/internal/repo"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestProviderCredentialAPIAuthorizationAndURLs(t *testing.T) {
	st, err := repo.OpenTest()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.PutUser(domain.User{LoginName: "provider-alice"}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutUser(domain.User{LoginName: "provider-bob"}); err != nil {
		t.Fatal(err)
	}
	alice, _ := st.UserByLoginName("provider-alice")
	authClient := auth.NewClient(auth.Config{SessionSecret: "provider-test-secret"})
	s := &Server{deps: &deps{repo: st, authClient: authClient}}
	s.users = &usersHandlers{deps: s.deps}
	g := gin.New()
	g.PUT("/api/users/:id/credentials", s.users.user)
	for _, tc := range []struct {
		name, login, body string
		code              int
	}{
		{"guest", "", "{\"codexApiKey\":\"secret\"}", 403},
		{"other", "provider-bob", "{\"claudeCodeApiKey\":\"secret\"}", 403},
		{"guest-platform", "", "{\"giteeToken\":\"test-secret\",\"gitlabToken\":\"test-secret\"}", 403},
		{"other-platform", "provider-bob", "{\"giteeTokenClear\":true,\"gitlabTokenClear\":true}", 403},
		{"invalid", "provider-alice", "{\"codexApiKey\":\"secret\",\"codexBaseUrl\":\"file:///tmp/x\"}", 400},
		{"url-secret", "provider-alice", "{\"claudeCodeBaseUrl\":\"https://user:secret@example.com\"}", 400},
		{"query", "provider-alice", "{\"codexBaseUrl\":\"https://example.com?key=secret\"}", 400},
		{"valid", "provider-alice", "{\"codexApiKey\":\"codex-test-secret\",\"codexBaseUrl\":\"http://localhost:9999/v1\",\"claudeCodeApiKey\":\"claude-test-secret\",\"claudeCodeBaseUrl\":\"https://example.com/anthropic\",\"giteeToken\":\"gitee-test-secret\",\"gitlabToken\":\"gitlab-test-secret\"}", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/users/%d/credentials", alice.ID), strings.NewReader(tc.body))
			if tc.login != "" {
				identity, ok := st.UserByLoginName(tc.login)
				if !ok {
					t.Fatalf("missing test identity %s", tc.login)
				}
				token, err := authClient.SignUser(&auth.User{ID: identity.ID, LoginName: identity.LoginName, AuthTime: time.Now().UTC()})
				if err != nil {
					t.Fatal(err)
				}
				req.AddCookie(&http.Cookie{Name: authCookieName, Value: token})
			}
			rec := httptest.NewRecorder()
			g.ServeHTTP(rec, req)
			if rec.Code != tc.code {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "test-secret") {
				t.Fatal("API leaked key")
			}
			if tc.code == 200 {
				var public domain.User
				if err := json.Unmarshal(rec.Body.Bytes(), &public); err != nil {
					t.Fatal(err)
				}
				if !public.CodexAPIKeyConfigured || !public.ClaudeCodeAPIKeyConfigured || !public.GiteeTokenConfigured || !public.GitLabTokenConfigured {
					t.Fatal("configured state missing")
				}
				stored, _ := st.User(alice.ID)
				if stored.CodexAPIKey != "codex-test-secret" || stored.ClaudeCodeAPIKey != "claude-test-secret" || stored.GiteeToken != "gitee-test-secret" || stored.GitLabToken != "gitlab-test-secret" {
					t.Fatal("provider keys were not stored")
				}
			}
		})
	}
}

func TestPlatformRepositoryBindings(t *testing.T) {
	f := cvBNewHTTPFixture(t)
	for _, tc := range []struct{ url, provider, identity string }{
		{"https://gitee.com/team/repo.git", "gitee", "gitee:team/repo"},
		{"https://gitlab.com/group/subgroup/repo", "gitlab", "gitlab:group/subgroup/repo"},
	} {
		resp := cvBRequest(t, f, http.MethodPost, "/api/repositories", map[string]string{"projectId": "platform", "repoUrl": tc.url}, true)
		var repository domain.Repository
		if resp.Code != http.StatusOK || json.Unmarshal(resp.Body.Bytes(), &repository) != nil || repository.Provider != tc.provider || repository.GongfengProjectID != tc.identity || repository.DefaultBranch != "main" {
			t.Fatalf("%s repository was not saved correctly: HTTP %d", tc.provider, resp.Code)
		}
	}
}
