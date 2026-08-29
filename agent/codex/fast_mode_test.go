package codex

import (
	"reflect"
	"testing"

	"github.com/chenhg5/cc-connect/core"
)

func TestSupportsFastModeModel(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{model: "gpt-5.6-luna", want: true},
		{model: "gpt-5.6-sol", want: true},
		{model: "gpt-5.6-terra", want: true},
		{model: "gpt-5.5", want: false},
		{model: "gpt-5.4", want: false},
		{model: "openai/gpt-5.6-luna", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := supportsFastModeModel(tt.model); got != tt.want {
				t.Fatalf("supportsFastModeModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestFastModeStatusRequiresOpenAIProviderAndSupportedModel(t *testing.T) {
	agent := &Agent{
		model: "gpt-5.6-sol",
		providers: []core.ProviderConfig{
			{Name: "openrouter", Model: "gpt-5.6-sol"},
			{Name: "openai", Model: "gpt-5.6-sol"},
		},
		activeIdx: 0,
		fastMode:  true,
	}

	status := agent.FastModeStatus()
	if status.Available {
		t.Fatal("FastModeStatus available with openrouter provider, want false")
	}

	agent.activeIdx = 1
	status = agent.FastModeStatus()
	if !status.Available {
		t.Fatal("FastModeStatus unavailable with openai/gpt-5.6-sol, want true")
	}
	if !status.Enabled {
		t.Fatal("FastModeStatus enabled = false, want true")
	}
	wantModels := []string{"gpt-5.6-luna", "gpt-5.6-sol", "gpt-5.6-terra"}
	if !reflect.DeepEqual(status.SupportedModels, wantModels) {
		t.Fatalf("supported models = %v, want %v", status.SupportedModels, wantModels)
	}

	agent.providers[1].Model = "gpt-5.4"
	if status := agent.FastModeStatus(); status.Available {
		t.Fatal("FastModeStatus available with gpt-5.4, want false")
	}
}
