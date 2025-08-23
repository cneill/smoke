package smoke

import "github.com/cneill/smoke/pkg/llms"

type AssistantResponseMessage struct {
	Message *llms.Message
}

type AssistantErrorMessage struct {
	Err error
}
