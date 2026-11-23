package domain

import "testing"

func TestPlatformRepositoryURLs(t *testing.T) {
	for _, tc := range []struct{ input, identity, clone string }{
		{"https://gitee.com/team/repo.git", "gitee:team/repo", "https://gitee.com/team/repo.git"},
		{"gitee:team/repo", "gitee:team/repo", "https://gitee.com/team/repo.git"},
		{"gitee.com/team/repo", "gitee:team/repo", "https://gitee.com/team/repo.git"},
		{"https://gitlab.com/group/subgroup/repo/-/tree/main", "gitlab:group/subgroup/repo", "https://gitlab.com/group/subgroup/repo.git"},
		{"gitlab:group/subgroup/repo", "gitlab:group/subgroup/repo", "https://gitlab.com/group/subgroup/repo.git"},
		{"gitlab.com/group/subgroup/repo", "gitlab:group/subgroup/repo", "https://gitlab.com/group/subgroup/repo.git"},
	} {
		got, ok := ParseRepositoryURL(tc.input)
		if !ok || got.Project != tc.identity || got.CloneURL != tc.clone {
			t.Fatalf("incorrect repository normalization: %q", tc.input)
		}
	}
	for _, raw := range []string{"https://gitee.com.attacker.invalid/team/repo", "https://user:secret@gitlab.com/team/repo", "https://gitlab.com/group/../repo", "https://gitee.com/team"} {
		if _, ok := ParseRepositoryURL(raw); ok {
			t.Fatal("invalid hosting URL accepted")
		}
	}
}

func TestPublicRepositoryAndPullRequestIdentity(t *testing.T) {
	for _, raw := range []string{"https://github.com/Tencent/LoopForge", "git@github.com:Tencent/LoopForge.git", "github:Tencent/LoopForge", "https://github.com/Tencent/LoopForge/tree/main"} {
		got, ok := ParseRepositoryURL(raw)
		if !ok || got.Project != "github:Tencent/LoopForge" || got.CloneURL != "https://github.com/Tencent/LoopForge.git" {
			t.Fatalf("%s: %+v %v", raw, got, ok)
		}
	}
	for _, raw := range []string{"https://github.com.attacker.invalid/owner/repo", "https://user:secret@github.com/owner/repo", "https://github.com/../repo", "https://github.com/owner"} {
		if got, ok := ParseRepositoryURL(raw); ok {
			t.Fatalf("invalid reference accepted: %s %+v", raw, got)
		}
	}
	p, n, u, ok := ParseGongfengMR("Delivered https://github.com/Tencent/LoopForge/pull/123")
	if !ok || p != "github:Tencent/LoopForge" || n != 123 || u != "https://github.com/Tencent/LoopForge/pull/123" {
		t.Fatalf("PR: %s %d %s %v", p, n, u, ok)
	}
}
