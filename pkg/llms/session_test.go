package llms_test

import (
	"testing"

	"github.com/cneill/smoke/pkg/llms"

	"github.com/stretchr/testify/assert"
)

func TestSession_ReplaceMessages(t *testing.T) { //nolint:funlen
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
