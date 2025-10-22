package history_test

import (
	"fmt"
	"testing"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/models/history"

	"github.com/stretchr/testify/assert"
)

func TestLog_AddMessage_Update(t *testing.T) {
	t.Parallel()

	id := "testID"
	log := history.NewLog()

	baseOpts := []llms.MessageOpt{
		llms.WithID(id),
		llms.WithRole(llms.RoleUser),
	}

	newMessage := func(opts ...llms.MessageOpt) *llms.Message {
		return llms.NewMessage(append(baseOpts, opts...)...)
	}

	messages := []*llms.Message{
		newMessage(llms.WithTextContent("hello")),
		newMessage(llms.WithTextContent("hello there")),
		newMessage(llms.WithTextContent("hello there, beautiful")),
	}

	for _, message := range messages {
		log.AddMessage(message)
	}

	log.AddMessage(fmt.Errorf("irrelevant error"))

	resultMsgs := log.Messages()

	assert.Len(t, resultMsgs, 2)

	llmMsg, ok := resultMsgs[0].(*llms.Message)
	assert.True(t, ok)
	assert.Equal(t, id, llmMsg.ID)
	assert.Equal(t, "hello there, beautiful", llmMsg.TextContent)

	_, isErr := resultMsgs[1].(error)
	assert.True(t, isErr)

	log.AddMessage(
		llms.NewMessage(
			llms.WithID("unknown"),
			llms.WithTextContent("unknown id"),
		),
	)

	assert.Len(t, log.Messages(), 3)
}
