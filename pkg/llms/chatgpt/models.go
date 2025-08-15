package chatgpt

import (
	"log/slog"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/openai/openai-go/v2"
)

func GetModel(search string, defaultModel openai.ChatModel) openai.ChatModel {
	aliases := llms.ModelAliases[openai.ChatModel]{
		openai.ChatModelGPT4o:    []string{"4", "4o", "gpt4o", "gpt-4o"},
		openai.ChatModelGPT4_1:   []string{"4.1", "gpt4.1", "gpt-4.1"},
		openai.ChatModelGPT5:     []string{"5", "gpt5", "gpt-5"},
		openai.ChatModelGPT5Mini: []string{"5-mini", "gpt5-mini", "gpt-5-mini"},
		openai.ChatModelO3:       []string{"o3", "gpto3", "gpt-o3"},
		openai.ChatModelO3Mini:   []string{"o3-mini", "gpto3-mini", "gpt-o3-mini"},
	}

	if model := aliases.Match(search); model != "" {
		return model
	}

	slog.Warn("model not found, using default", "search", search)

	return defaultModel
}
