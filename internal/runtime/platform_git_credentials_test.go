package runtime

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGitEnvironmentScopesPlatformCredentials(t *testing.T) {
	workspace := t.TempDir()
	environment, err := GitEnvironment(nil, workspace, "github-secret", map[string]string{"gitee.com": "oauth2:gitee-secret", "gitlab.com": ""})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ host, token string }{{"gitee.com", "gitee-secret"}, {"github.com", "github-secret"}, {"gitlab.com", ""}} {
		cmd := exec.Command("git", "credential", "fill")
		cmd.Dir = workspace
		cmd.Env = os.Environ()
		for key, value := range environment {
			cmd.Env = upsertEnv(cmd.Env, key, value)
		}
		cmd.Env = upsertEnv(cmd.Env, "GIT_CONFIG_GLOBAL", os.DevNull)
		cmd.Env = upsertEnv(cmd.Env, "GIT_CONFIG_NOSYSTEM", "1")
		cmd.Stdin = strings.NewReader("protocol=https\nhost=" + tc.host + "\n\n")
		out, err := cmd.CombinedOutput()
		if tc.token == "" {
			if err == nil || strings.Contains(string(out), "github-secret") {
				t.Fatal("unconfigured platform received a fallback token")
			}
		} else if err != nil || !strings.Contains(string(out), "password="+tc.token+"\n") {
			t.Fatal("incorrect runtime credential for " + tc.host)
		}
	}
	askpass, err := os.ReadFile(environment["GIT_ASKPASS"])
	if err != nil || strings.Contains(string(askpass), "secret") {
		t.Fatal("runtime askpass file contains a token or is missing")
	}
}

func TestPlatformCredentialsAreRedactedAndNotInherited(t *testing.T) {
	command := compactCommand([]string{"docker", "exec", "-e", "AGENTFLOW_GIT_HOST_PASSWORD_0=platform-secret", "sandbox", "git"})
	if strings.Contains(command, "platform-secret") {
		t.Fatal("platform credential leaked into command telemetry")
	}
	base := controlledBaseEnvironment([]string{"PATH=/bin", "AGENTFLOW_GIT_HOST_PASSWORD_0=platform-secret", "AGENTFLOW_GIT_HOST_USER_0=oauth2"})
	if len(base) != 1 || base[0] != "PATH=/bin" {
		t.Fatal("child inherited a previous platform credential")
	}
}
