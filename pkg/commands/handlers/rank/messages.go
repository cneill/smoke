package rank

import "github.com/cneill/smoke/pkg/commands"

type RequestMessage struct {
	commands.MessageType

	PromptMessage commands.PromptMessage
	Iteration     int
	BatchIdx      int
	Batch         Items
	Description   string
	ResponseChan  chan<- ResponseMessage `json:"-"`
	Retries       int
}

type ResponseMessage struct {
	RequestMessage

	Message string
	Err     error
}
