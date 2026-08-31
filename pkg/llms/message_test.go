package llms_test

import (
	"testing"

	"github.com/cneill/smoke/pkg/llms"
	"github.com/stretchr/testify/assert"
)

func TestMessageTextBlockUpdate(t *testing.T) {
	t.Parallel()

	msg := llms.NewMessage(
		llms.WithID("123"),
		llms.WithTextContent("start"),
	)

	assert.Equal(t, "start", msg.TextContent())

	updated := msg.Update(
		llms.WithTextContent("updated"),
	)

	assert.Equal(t, "123", updated.ID)
	assert.Equal(t, "updated", updated.TextContent())
}
