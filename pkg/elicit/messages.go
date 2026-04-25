package elicit

import (
	"fmt"
	"strings"
)

type Message interface {
	isElicitMessage()
}

// RequestMessage contains the details of the question asked by the model.
type RequestMessage struct {
	Question string
	Options  []string
}

func (r RequestMessage) OK() error {
	switch {
	case strings.TrimSpace(r.Question) == "":
		return fmt.Errorf("missing question")
	case len(r.Options) == 0 || len(r.Options) > 5:
		return fmt.Errorf("options must contain between 1 and 5 items")
	}

	for i, option := range r.Options {
		if strings.TrimSpace(option) == "" {
			return fmt.Errorf("option %d must not be empty", i+1)
		}
	}

	return nil
}

func (r RequestMessage) String() string {
	builder := &strings.Builder{}
	fmt.Fprintln(builder, r.Question)

	builder.WriteRune('\n')

	for i, option := range r.Options {
		fmt.Fprintf(builder, "%d. %s\n", i+1, option)
	}

	fmt.Fprint(builder, "none. None of the above")

	return builder.String()
}

func (r RequestMessage) isElicitMessage() {}

// UserInputMessage contains the raw content submitted by the user to answer an elicited question.
type UserInputMessage struct {
	Content string
}

func (u UserInputMessage) isElicitMessage() {}

// UserCanceledMessage signals that the user cancelled the elicitation.
type UserCanceledMessage struct{}

func (u UserCanceledMessage) String() string {
	return "User canceled elicitation request"
}

func (u UserCanceledMessage) isElicitMessage() {}

// UserResponseMessage wraps the Response parsed from the user's input.
type UserResponseMessage struct {
	*Response
}

func (u UserResponseMessage) String() string {
	var str string

	switch {
	case u.Canceled:
		str = "Canceled"
	case u.NoneOfTheAbove:
		str = "None of the above"
		if u.Elaboration != "" {
			str += ": " + u.Elaboration
		}
	default:
		str = u.Selected
		if u.Elaboration != "" {
			str += ": " + u.Elaboration
		}
	}

	return str
}

func (u UserResponseMessage) isElicitMessage() {}
