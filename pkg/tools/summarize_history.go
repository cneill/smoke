package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	smokefs "github.com/cneill/smoke/pkg/fs"
)

const (
	SummarizeHistoryMessage = "summary_message"
)

type SummarizeHistoryTool struct {
	ProjectPath     string
	SessionName     string
	HistoryFileName string
}

func NewSummarizeHistoryTool(projectPath, sessionName string) Tool {
	return &SummarizeHistoryTool{
		ProjectPath:     projectPath,
		SessionName:     sessionName,
		HistoryFileName: sessionName + "_history_summary.json",
	}
}

func (s *SummarizeHistoryTool) Name() string { return ToolSummarizeHistory }
func (s *SummarizeHistoryTool) Description() string {
	examples := CollectExamples(s.Examples()...)

	return "Accepts a single string message containing the model's summarization of current session history." + examples
}

func (s *SummarizeHistoryTool) Examples() Examples {
	// TODO: give more/better examples here
	return Examples{
		{
			Description: "Summarize the current session history with a compact string",
			Args: Args{
				SummarizeHistoryMessage: "Previous conversation involved planning a new Go tool for history " +
					"summarization, discussing approaches using files for session state without modifying existing " +
					"tools.",
			},
		},
	}
}

func (s *SummarizeHistoryTool) Params() Params {
	return Params{
		{
			Key:         SummarizeHistoryMessage,
			Description: "The model's summarization of the current session history to compress it.",
			Type:        ParamTypeString,
			Required:    true,
		},
	}
}

func (s *SummarizeHistoryTool) Run(_ context.Context, args Args) (string, error) {
	summary := args.GetString(SummarizeHistoryMessage)
	if summary == nil || *summary == "" {
		return "", fmt.Errorf("%w: no summary message supplied", ErrArguments)
	}

	fullPath, err := smokefs.GetRelativePath(s.ProjectPath, s.HistoryFileName)
	if err != nil {
		return "", fmt.Errorf("%w: failed to construct history summary file path: %w", ErrFileSystem, err)
	}

	if _, err := os.Stat(fullPath); err == nil {
		return "", fmt.Errorf("%w: a previous history summarization file already exists and must be removed first", ErrFileSystem)
	}

	data := map[string]string{"summary": *summary}

	bytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("%w: failed to marshal summary data: %w", ErrInvalidJSON, err)
	}

	err = os.WriteFile(fullPath, bytes, 0o644)
	if err != nil {
		return "", fmt.Errorf("%w: failed to write history summary file: %w", ErrFileSystem, err)
	}

	// TODO: include session somehow so we can get session message/token history
	slog.Debug("wrote history summarization to file", "path", fullPath, "length", len(bytes))

	return "History summarized.", nil
}
