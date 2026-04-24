package history

import "github.com/cneill/smoke/pkg/elicit"

type ElicitPromptMessage struct {
	Request elicit.Request
}

type ElicitResponseMessage struct {
	Response string
}
