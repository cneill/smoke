package gemini //nolint:testpackage // Tests exercise the unexported client constructor.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	genai "google.golang.org/genai/interactions"
)

// The standalone interactions client has no default API version, so a missing WithAPIVersion produces requests to
// "//interactions", which Google answers with an HTML 404.
func TestClientSendsVersionedPathAndAPIKey(t *testing.T) {
	t.Parallel()

	var (
		gotPath   string
		gotAPIKey string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("X-Goog-Api-Key")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"interaction","status":"completed","steps":[]}`))
	}))
	t.Cleanup(server.Close)

	conv := testConversation(t)
	conv.client = newClient("test-key", genai.WithServerURL(server.URL))

	resp, err := conv.client.Interactions.Create(t.Context(), conv.getCreateParams(false))
	require.NoError(t, err)
	require.NotNil(t, resp.Interaction)

	assert.Equal(t, "/"+apiVersion+"/interactions", gotPath)
	assert.Equal(t, "test-key", gotAPIKey)
}

// With Store=false the server doesn't return a usable interaction ID. Every streamed delta and the final message must
// still share one valid ID, or the history view can't replace the streaming placeholder (or rejects it outright).
func TestSendStreamUsesOneLocalMessageID(t *testing.T) {
	t.Parallel()

	frames := []string{
		`{"event_type":"interaction.created","interaction":{"id":"","status":"in_progress"}}`,
		`{"event_type":"step.start","index":0,"step":{"type":"model_output"}}`,
		`{"event_type":"step.delta","index":0,"delta":{"type":"text","text":"Hel"}}`,
		`{"event_type":"step.delta","index":0,"delta":{"type":"text","text":"lo"}}`,
		`{"event_type":"step.stop","index":0}`,
		`{"event_type":"interaction.completed","interaction":{"id":"","status":"completed"}}`,
		`[DONE]`,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		for _, frame := range frames {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", frame)
		}
	}))
	t.Cleanup(server.Close)

	conv := testConversation(t)
	conv.client = newClient("test-key", genai.WithServerURL(server.URL))

	events, err := collectEvents(t, conv, conv.sendStream)
	require.NoError(t, err)
	require.NotEmpty(t, events)

	final, ok := events[len(events)-1].(llms.EventFinalMessage)
	require.True(t, ok, "last event should be the final message, got %T", events[len(events)-1])
	require.NoError(t, final.Message.OK())
	assert.Equal(t, "Hello", final.Message.TextContent())

	var text strings.Builder

	for _, event := range events {
		if delta, ok := event.(llms.EventTextDelta); ok {
			assert.Equal(t, final.Message.ID, delta.ID)
			text.WriteString(delta.Text)
		}
	}

	assert.Equal(t, "Hello", text.String())
}
