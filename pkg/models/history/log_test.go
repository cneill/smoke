package history_test

import (
	"fmt"
	"testing"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/models/history"

	"github.com/stretchr/testify/assert"
)

func TestLog_AddMessage_Chunked(t *testing.T) {
	t.Parallel()

	id := "testID"
	log := history.NewLog()

	baseOpts := []llms.MessageOpt{
		llms.WithID(id),
		llms.WithRole(llms.RoleUser),
		llms.WithIsStreamed(true),
	}

	messages := []*llms.Message{
		llms.NewMessage(append(baseOpts, llms.WithContent("hello"), llms.WithIsInitial(true), llms.WithIsChunk(true))...),
		llms.NewMessage(append(baseOpts, llms.WithContent("hello there"), llms.WithIsChunk(true))...),
		llms.NewMessage(append(baseOpts, llms.WithContent("hello there, beautiful"), llms.WithIsFinalized(true))...),
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
	assert.Equal(t, "hello there, beautiful", llmMsg.Content)

	log.AddMessage(
		llms.NewMessage(append(baseOpts, llms.WithContent("hello there invalid"), llms.WithIsChunk(true))...),
	)

	// make sure we didn't modify the finalized message
	assert.Len(t, resultMsgs, 2)

	currentLLMMessage, ok := resultMsgs[0].(*llms.Message)
	assert.True(t, ok)
	assert.Equal(t, id, currentLLMMessage.ID)
	assert.Equal(t, "hello there, beautiful", currentLLMMessage.Content)

	log.AddMessage(
		llms.NewMessage(
			llms.WithID("unknown"),
			llms.WithIsStreamed(true),
			llms.WithIsChunk(true),
			llms.WithContent("unknown id"),
		),
	)

	// make sure we threw away the chunk message with an unknown ID
	assert.Len(t, resultMsgs, 2)
}
