package history_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/commands/handlers/mode"
	"github.com/cneill/smoke/pkg/elicit"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/models/history"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	helloText          = "hello"
	internalErrorTitle = "⛔ Internal error"
)

type bubbleExpectation struct {
	name         string
	item         any
	wantTitle    string
	wantText     string
	markdown     bool
	structured   bool
	empty        bool
	contentEmpty bool
}

func TestBubbleForHistoryItem(t *testing.T) {
	t.Parallel()

	styles := history.InitStyles(80)
	now := time.Date(2026, 5, 25, 14, 0, 0, 0, time.UTC)
	tests := bubbleForHistoryItemTests(now)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bubble := history.BubbleForHistoryItem(tt.item, styles)

			assert.Equal(t, tt.wantTitle, bubble.TitleText)
			assert.Equal(t, tt.wantText, bubble.ContentText())
			assert.Equal(t, tt.markdown, bubble.ContentIsMarkdown())
			assert.Equal(t, tt.structured, bubble.ContentIsStructured())
			assert.Equal(t, tt.empty, bubble.IsEmpty())
			assert.Equal(t, tt.contentEmpty, bubble.ContentIsEmpty())
		})
	}
}

func bubbleForHistoryItemTests(now time.Time) []bubbleExpectation { //nolint:funlen
	return []bubbleExpectation{
		{
			name: "user llm message",
			item: func() *llms.Message {
				msg := llms.NewMessage(
					llms.WithID("msg-1"),
					llms.WithRole(llms.RoleUser),
					llms.WithTextContent(helloText),
				)
				msg.Added = now

				return msg
			}(),
			wantTitle: "👤 User",
			wantText:  helloText,
			markdown:  true,
		},
		{
			name: "history update with markdown text",
			item: commands.HistoryUpdateMessage{
				PromptMessage: commands.PromptMessage{Command: "help"},
				Message:       "**hello**",
			},
			wantTitle: "⚙️ Help command result",
			wantText:  "**hello**",
			markdown:  true,
		},
		{
			name: "history update with structured content",
			item: commands.HistoryUpdateMessage{
				PromptMessage: commands.PromptMessage{Command: "info"},
				Content: &uimsg.HistoryContent{
					Blocks: []uimsg.HistoryBlock{
						{Type: uimsg.HistoryBlockText, Text: helloText},
					},
				},
			},
			wantTitle:  "⚙️ Info command result",
			structured: true,
		},
		{
			name:         "mode message",
			item:         mode.Message{Mode: "chat", Message: "Switched"},
			wantTitle:    "Chat mode",
			empty:        false,
			contentEmpty: true,
		},
		{
			name:      "elicit request",
			item:      elicit.RequestMessage{Question: "Proceed?", Options: []string{"Yes"}},
			wantTitle: "Question",
			wantText:  "Proceed?\n\n1. Yes\nnone. None of the above",
		},
		{
			name:      "uimsg error",
			item:      uimsg.ToError(fmt.Errorf("boom")),
			wantTitle: "⛔ Error",
			wantText:  "boom",
		},
		{
			name:      "plain string",
			item:      "mystery",
			wantTitle: "❓ Unknown message",
			wantText:  "mystery",
		},
		{
			name:      "unsupported type becomes internal error bubble",
			item:      123,
			wantTitle: internalErrorTitle,
			wantText:  "history received unsupported message type int",
		},
		{
			name: "empty assistant message = empty bubble",
			item: func() *llms.Message {
				msg := llms.NewMessage(
					llms.WithID("msg-1"),
					llms.WithRole(llms.RoleAssistant),
					llms.WithTextContent(""),
					llms.WithLLMInfo(&llms.LLMInfo{
						Type:      llms.LLMTypeOllama,
						ModelName: "happy model",
					}),
				)
				msg.Added = now

				return msg
			}(),
			empty:        true,
			contentEmpty: true,
		},
	}
}

func TestRendererRenderBubble(t *testing.T) {
	t.Parallel()

	renderer, err := history.NewRenderer(80)
	require.NoError(t, err)

	structured := renderer.RenderBubble(history.Bubble{
		Style:     renderer.Styles().CommandBubble,
		TitleText: "Structured",
		Content: history.StructuredContent(&uimsg.HistoryContent{
			Blocks: []uimsg.HistoryBlock{{
				Type:  uimsg.HistoryBlockFields,
				Title: "Session info",
				Fields: []uimsg.HistoryField{
					uimsg.NewField("Provider", "ollama"),
				},
			}},
		}),
	})

	assert.Contains(t, structured, "Session info")
	assert.Contains(t, structured, "Provider")

	plain := renderer.RenderBubble(history.Bubble{
		Style:     renderer.Styles().SessionBubble,
		TitleText: "Plain",
		Content:   history.PlainTextContent(strings.Repeat("word ", 20)),
	})

	assert.Contains(t, plain, "Plain")
	assert.Contains(t, plain, "word")
}
