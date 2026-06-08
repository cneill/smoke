package llms_test

import (
	"testing"

	"github.com/cneill/smoke/pkg/llms"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionMessageHandling(t *testing.T) {
	t.Parallel()

	session := &llms.Session{}
	msg1 := llms.NewMessage(
		llms.WithID("id-user-1"),
		llms.WithRole(llms.RoleUser),
		llms.WithTextContent("say hello"),
	)

	msg2 := llms.NewMessage(
		llms.WithID("id-assistant-1"),
		llms.WithRole(llms.RoleAssistant),
		llms.WithTextContent("hello"),
	)

	msg3 := llms.NewMessage(
		llms.WithID("id-assistant-2"),
		llms.WithRole(llms.RoleAssistant),
		llms.WithTextContent("bye"),
	)

	require.NoError(t, session.AddMessage(msg1))
	require.NoError(t, session.AddMessage(msg2))
	require.NoError(t, session.AddMessage(msg3))

	last := session.Last()
	assert.Equal(t, "id-assistant-2", last.ID)
	assert.Equal(t, llms.RoleAssistant, last.Role)
	assert.NotZero(t, last.Added)

	lastAssistant := session.LastByRole(llms.RoleAssistant)
	assert.Equal(t, "id-assistant-2", lastAssistant.ID)

	lastAssistantRun := session.LastRunByRole(llms.RoleAssistant)
	assert.Len(t, lastAssistantRun, 2)
	assert.Equal(t, "id-assistant-1", lastAssistantRun[0].ID)
	assert.Equal(t, "id-assistant-2", lastAssistantRun[1].ID)

	assert.Nil(t, session.LastByRole(llms.RoleSystem))

	counts := session.MessageCount()
	assert.Equal(t, 2, counts.AssistantMessages)
	assert.Equal(t, 1, counts.UserMessages)
	assert.Equal(t, 0, counts.ToolCallMessages)
}

func TestSession_ReplaceMessages(t *testing.T) {
	t.Parallel()

	message1 := llms.NewMessage(llms.WithID("id-1"))
	message2 := llms.NewMessage(llms.WithID("id-2"))
	message3 := llms.NewMessage(llms.WithID("id-3"))

	tests := []struct {
		name            string
		startMessages   []*llms.Message
		searchMessages  []*llms.Message
		replaceMessages []*llms.Message
		finalMessages   []*llms.Message
	}{
		{
			name:            "empty_session_start",
			startMessages:   []*llms.Message{},
			searchMessages:  []*llms.Message{},
			replaceMessages: []*llms.Message{message1, message2},
			finalMessages:   []*llms.Message{message1, message2},
		},
		{
			name:            "search_not_found",
			startMessages:   []*llms.Message{message1},
			searchMessages:  []*llms.Message{message3},
			replaceMessages: []*llms.Message{message2},
			finalMessages:   []*llms.Message{message1, message2},
		},
		{
			name:            "replace_one",
			startMessages:   []*llms.Message{message1, message2},
			searchMessages:  []*llms.Message{message1},
			replaceMessages: []*llms.Message{message3},
			finalMessages:   []*llms.Message{message3, message2},
		},
		{
			name:            "replace_all",
			startMessages:   []*llms.Message{message1, message2},
			searchMessages:  []*llms.Message{message1, message2},
			replaceMessages: []*llms.Message{message3},
			finalMessages:   []*llms.Message{message3},
		},
		{
			name:            "remove_two",
			startMessages:   []*llms.Message{message1, message2, message3},
			searchMessages:  []*llms.Message{message1, message2},
			replaceMessages: []*llms.Message{},
			finalMessages:   []*llms.Message{message3},
		},
		{
			name:            "remove_all",
			startMessages:   []*llms.Message{message1, message2, message3},
			searchMessages:  []*llms.Message{message1, message2, message3},
			replaceMessages: []*llms.Message{},
			finalMessages:   []*llms.Message{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			session := &llms.Session{
				Messages: test.startMessages,
			}

			session.ReplaceMessages(test.searchMessages, test.replaceMessages)

			assert.Equal(t, test.finalMessages, session.Messages, "session's messages didn't match expected ones after replace")
		})
	}
}
