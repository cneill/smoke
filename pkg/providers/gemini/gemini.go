// Package gemini contains an implementation of [llms.LLM] for Google's Gemini, using the Interactions API.
package gemini

import (
	"context"
	"fmt"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/providers/base"
	genai "google.golang.org/genai/interactions"
	"google.golang.org/genai/interactions/models/components"
)

// apiVersion fills the {api_version} segment of every Interactions API path. The standalone interactions client leaves
// it unset (only the root genai.Client supplies one), which produces requests to "//interactions" and an HTML 404. The
// SDK exports no constant for it; "v1beta" matches the Interactions OpenAPI document version.
const apiVersion = "v1beta"

type Gemini struct {
	config *llms.Config
	client *genai.GenAI
}

func New(config *llms.Config) (llms.LLM, error) {
	if err := config.OK(); err != nil {
		return nil, fmt.Errorf("error with Gemini options: %w", err)
	}

	gemini := &Gemini{
		config: config,
		client: newClient(config.APIKey),
	}

	return gemini, nil
}

// newClient builds the Interactions API client. Extra options exist so tests can redirect the server URL.
func newClient(apiKey string, opts ...genai.SDKOption) *genai.GenAI {
	baseOpts := []genai.SDKOption{ //nolint:prealloc
		genai.WithSecurity(components.Security{APIKey: &apiKey}),
		genai.WithAPIVersion(apiVersion),
	}

	return genai.New(append(baseOpts, opts...)...)
}

func (g *Gemini) LLMInfo() *llms.LLMInfo {
	return &llms.LLMInfo{
		Type:      llms.LLMTypeGemini,
		ModelName: g.config.ModelInfo.Model,
	}
}

func (g *Gemini) RequiresSessionSystem() bool { return false }

func (g *Gemini) StartConversation(ctx context.Context, session *llms.Session) llms.Conversation {
	// Two-step construction: the send funcs close over conv, so we build conv first, then wire the
	// base in.
	conv := &conversation{client: g.client}

	baseConv, newCtx, err := base.NewConversation(ctx, &base.ConversationOpts{
		Session:      session,
		LLMInfo:      g.LLMInfo(),
		Config:       g.config,
		Stream:       !g.config.NoStream,
		SendStream:   conv.sendStream,
		SendNoStream: conv.sendNoStream,
	})
	if err != nil {
		// Config was already validated in New(), so this should never happen.
		panic(fmt.Sprintf("gemini: failed to create base conversation: %v", err))
	}

	conv.Conversation = baseConv

	go conv.Start(newCtx)

	return conv
}
