package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenhg5/cc-connect/core"
)

func TestEnsureCodexProviderConfig_CreatesNewFile(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".codex")

	err := ensureCodexProviderConfig(home, "shengsuanyun",
		"https://router.shengsuanyun.com/api/v1", "responses",
		map[string]string{"HTTP-Referer": "https://openai.com/zh-Hans-CN/codex/", "X-Title": "CodeX"})
	if err != nil {
		t.Fatalf("ensureCodexProviderConfig: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	content := string(data)

	for _, want := range []string{
		`[model_providers.shengsuanyun]`,
		`env_key = "OPENAI_API_KEY"`,
		`wire_api = "responses"`,
		`base_url = "https://router.shengsuanyun.com/api/v1"`,
		`[model_providers.shengsuanyun.http_headers]`,
		`"HTTP-Referer" = "https://openai.com/zh-Hans-CN/codex/"`,
		`"X-Title" = "CodeX"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("config.toml missing %q\ngot:\n%s", want, content)
		}
	}
}

func TestEnsureCodexProviderConfig_UpdatesExistingSection(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".codex")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}

	initial := `model = "gpt-5.4"

[model_providers.shengsuanyun]
name = "shengsuanyun"
env_key = "OLD_KEY"
wire_api = "chat"

[some_other_section]
key = "value"
`
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ensureCodexProviderConfig(home, "shengsuanyun",
		"https://router.shengsuanyun.com/api/v1", "responses", nil)
	if err != nil {
		t.Fatalf("ensureCodexProviderConfig: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, `env_key = "OPENAI_API_KEY"`) {
		t.Errorf("updated config missing new env_key\ngot:\n%s", content)
	}
	if strings.Contains(content, `env_key = "OLD_KEY"`) {
		t.Errorf("updated config still has old env_key\ngot:\n%s", content)
	}
	if !strings.Contains(content, `wire_api = "responses"`) {
		t.Errorf("updated config missing new wire_api\ngot:\n%s", content)
	}
	if !strings.Contains(content, `[some_other_section]`) {
		t.Errorf("updated config lost other section\ngot:\n%s", content)
	}
	if !strings.Contains(content, `model = "gpt-5.4"`) {
		t.Errorf("updated config lost top-level key\ngot:\n%s", content)
	}
}

func TestEnsureCodexProviderConfig_DefaultEnvKey(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".codex")

	err := ensureCodexProviderConfig(home, "dmxapi", "https://www.dmxapi.cn/v1", "responses", nil)
	if err != nil {
		t.Fatalf("ensureCodexProviderConfig: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, `env_key = "OPENAI_API_KEY"`) {
		t.Errorf("config should contain default env_key OPENAI_API_KEY\ngot:\n%s", content)
	}
	if strings.Contains(content, "requires_openai_auth") {
		t.Errorf("config should NOT contain requires_openai_auth\ngot:\n%s", content)
	}
}

func TestEnsureCodexProviderConfig_PreservesOtherProviders(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".codex")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}

	initial := `[model_providers.other]
name = "other"
env_key = "OTHER_KEY"

[model_providers.other.http_headers]
"X-Custom" = "val"
`
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ensureCodexProviderConfig(home, "shengsuanyun", "", "responses", nil)
	if err != nil {
		t.Fatalf("ensureCodexProviderConfig: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, `[model_providers.other]`) {
		t.Errorf("lost other provider section\ngot:\n%s", content)
	}
	if !strings.Contains(content, `[model_providers.shengsuanyun]`) {
		t.Errorf("new provider not added\ngot:\n%s", content)
	}
}

func TestEnsureCodexProviderConfig_SkipsWhenEmpty(t *testing.T) {
	err := ensureCodexProviderConfig("", "", "", "", nil)
	if err != nil {
		t.Fatalf("unexpected error for empty name: %v", err)
	}
}

func TestStartSession_PreservesOAuthAuth(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".codex")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"oauth-access","refresh_token":"oauth-refresh"}}`)
	if err := os.WriteFile(filepath.Join(home, "auth.json"), original, 0o600); err != nil {
		t.Fatal(err)
	}

	agent := &Agent{
		workDir:   t.TempDir(),
		model:     "mimo-v2.5",
		mode:      "full-auto",
		backend:   "exec",
		cmd:       "codex",
		codexHome: home,
		providers: []core.ProviderConfig{{
			Name:    "mimo",
			APIKey:  "sk-test-key",
			BaseURL: "http://127.0.0.1:8193/v1",
		}},
		activeIdx: 0,
	}

	session, err := agent.StartSession(context.Background(), "")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	defer session.Close()

	data, err := os.ReadFile(filepath.Join(home, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Errorf("auth.json changed after API-key provider session\nwant: %s\ngot:  %s", original, data)
	}
}
