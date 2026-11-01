package codex

import (
	"context"
	"os"
	"testing"

	"github.com/chenhg5/cc-connect/core"
)

// writeCatalogHome creates a CODEX_HOME whose config.toml points at a
// model_catalog.json holding the given models, and returns the temp dir.
func writeCatalogHome(t *testing.T, models string) string {
	t.Helper()
	tmp := t.TempDir()
	if err := os.WriteFile(tmp+"/config.toml", []byte(`model_catalog_json = "model_catalog.json"`+"\n"), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}
	if err := os.WriteFile(tmp+"/model_catalog.json", []byte(models), 0o644); err != nil {
		t.Fatalf("write model_catalog.json: %v", err)
	}
	t.Setenv("CODEX_HOME", tmp)
	return tmp
}

// TestAvailableModels_ProviderModelsOverrideCatalog is the regression test for
// the bug where /model kept listing GPT models after switching to a non-OpenAI
// provider: a provider's configured model list must beat the global catalog.
func TestAvailableModels_ProviderModelsOverrideCatalog(t *testing.T) {
	writeCatalogHome(t, `{
  "models": [
    {"slug":"gpt-5.6","description":"OpenAI: GPT-5.6","visibility":"list","supported_in_api":true},
    {"slug":"gpt-6-astra","description":"OpenAI: GPT-6 Astra","visibility":"list","supported_in_api":true}
  ]
}`)

	a := &Agent{
		providers: []core.ProviderConfig{
			{Name: "deepseek", Model: "deepseek-flash", Models: []core.ModelOption{{Name: "deepseek-flash"}}},
		},
		activeIdx: 0,
	}

	models := a.AvailableModels(context.Background())
	if len(models) != 1 {
		t.Fatalf("models = %v, want exactly [deepseek-flash]", models)
	}
	if models[0].Name != "deepseek-flash" {
		t.Fatalf("models[0].Name = %q, want deepseek-flash", models[0].Name)
	}
}

// TestAvailableModels_ProviderModelsFollowActiveProvider checks that switching
// the active provider switches the offered models.
func TestAvailableModels_ProviderModelsFollowActiveProvider(t *testing.T) {
	writeCatalogHome(t, `{
  "models": [
    {"slug":"gpt-5.6","description":"OpenAI: GPT-5.6","visibility":"list","supported_in_api":true}
  ]
}`)

	a := &Agent{
		providers: []core.ProviderConfig{
			{Name: "deepseek", Models: []core.ModelOption{{Name: "deepseek-flash"}}},
			{Name: "mimo", Models: []core.ModelOption{{Name: "mimo-v2.5"}, {Name: "mimo-v2.5-pro"}}},
		},
		activeIdx: 0,
	}

	for _, tc := range []struct {
		idx  int
		want []string
	}{
		{idx: 0, want: []string{"deepseek-flash"}},
		{idx: 1, want: []string{"mimo-v2.5", "mimo-v2.5-pro"}},
	} {
		a.activeIdx = tc.idx
		models := a.AvailableModels(context.Background())
		if len(models) != len(tc.want) {
			t.Fatalf("activeIdx=%d models = %v, want %v", tc.idx, models, tc.want)
		}
		for i, want := range tc.want {
			if models[i].Name != want {
				t.Fatalf("activeIdx=%d models[%d] = %q, want %q", tc.idx, i, models[i].Name, want)
			}
		}
	}
}

// TestAvailableModels_FallsBackToCatalogWithoutProviderModels keeps the plain
// OpenAI setup working: no configured models → catalog list.
func TestAvailableModels_FallsBackToCatalogWithoutProviderModels(t *testing.T) {
	writeCatalogHome(t, `{
  "models": [
    {"slug":"gpt-5.6","description":"OpenAI: GPT-5.6","visibility":"list","supported_in_api":true}
  ]
}`)

	a := &Agent{
		providers: []core.ProviderConfig{{Name: "openai"}},
		activeIdx: 0,
	}

	models := a.AvailableModels(context.Background())
	if len(models) != 1 || models[0].Name != "gpt-5.6" {
		t.Fatalf("models = %v, want [gpt-5.6]", models)
	}
}

// TestAvailableModels_ConfiguredModelsGetCatalogDescriptions checks the
// enrichment: provider config carries only model ids, the catalog supplies the
// readable description.
func TestAvailableModels_ConfiguredModelsGetCatalogDescriptions(t *testing.T) {
	writeCatalogHome(t, `{
  "models": [
    {"slug":"deepseek-flash","display_name":"deepseek-flash","description":"DeepSeek: V4.1-Flash","visibility":"list","supported_in_api":true}
  ]
}`)

	a := &Agent{
		providers: []core.ProviderConfig{
			{Name: "deepseek", Models: []core.ModelOption{{Name: "deepseek-flash"}}},
		},
		activeIdx: 0,
	}

	models := a.AvailableModels(context.Background())
	if len(models) != 1 || models[0].Desc != "DeepSeek: V4.1-Flash" {
		t.Fatalf("models = %v, want desc %q", models, "DeepSeek: V4.1-Flash")
	}
}

func TestWithCatalogDescriptions(t *testing.T) {
	catalog := []core.ModelOption{
		{Name: "deepseek-flash", Desc: "DeepSeek: V4.1-Flash"},
		{Name: "gpt-5.6", Desc: "OpenAI: GPT-5.6"},
		{Name: "no-desc"},
	}

	t.Run("fills missing description only", func(t *testing.T) {
		in := []core.ModelOption{
			{Name: "deepseek-flash"},
			{Name: "gpt-5.6", Desc: "keep me"},
			{Name: "unknown-model"},
			{Name: "no-desc"},
		}
		got := withCatalogDescriptions(in, catalog)
		if len(got) != 4 {
			t.Fatalf("len = %d, want 4", len(got))
		}
		if got[0].Desc != "DeepSeek: V4.1-Flash" {
			t.Errorf("got[0].Desc = %q, want catalog desc", got[0].Desc)
		}
		if got[1].Desc != "keep me" {
			t.Errorf("got[1].Desc = %q, want provider desc preserved", got[1].Desc)
		}
		if got[2].Desc != "" {
			t.Errorf("got[2].Desc = %q, want empty for unknown model", got[2].Desc)
		}
		if got[3].Desc != "" {
			t.Errorf("got[3].Desc = %q, want empty (catalog entry has no desc)", got[3].Desc)
		}
	})

	t.Run("does not mutate provider config", func(t *testing.T) {
		in := []core.ModelOption{{Name: "deepseek-flash"}}
		_ = withCatalogDescriptions(in, catalog)
		if in[0].Desc != "" {
			t.Fatalf("input mutated: %+v", in[0])
		}
	})

	t.Run("empty inputs", func(t *testing.T) {
		if got := withCatalogDescriptions(nil, catalog); got != nil {
			t.Fatalf("empty models: got %v, want nil", got)
		}
		in := []core.ModelOption{{Name: "deepseek-flash"}}
		if got := withCatalogDescriptions(in, nil); len(got) != 1 || got[0].Desc != "" {
			t.Fatalf("empty catalog: got %v", got)
		}
	})
}
