package core

import (
	"strings"
	"testing"
)

func TestAgentFileSendPromptForLang(t *testing.T) {
	for _, lang := range []Language{LangEnglish, LangChinese, LangTraditionalChinese, LangJapanese, LangSpanish, LangAuto} {
		t.Run(string(lang), func(t *testing.T) {
			prompt := AgentFileSendPromptForLang(lang)
			markers := []string{"cc-connect send --file", "CC_PROJECT", "CC_SESSION_KEY"}
			if lang == LangChinese || lang == LangTraditionalChinese {
				markers = append(markers, "命令成功")
			} else {
				markers = append(markers, "successfully")
			}
			for _, marker := range markers {
				if !strings.Contains(prompt, marker) {
					t.Errorf("prompt missing %q: %q", marker, prompt)
				}
			}
			if len(prompt) > 2000 {
				t.Errorf("prompt is not compact: %d bytes", len(prompt))
			}
		})
	}

	if AgentFileSendPromptForLang(LangChinese) == AgentFileSendPromptForLang(LangEnglish) {
		t.Fatal("Chinese and English file-send prompts are identical")
	}
}
