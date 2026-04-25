package elicit

import (
	"fmt"
	"strconv"
	"strings"
)

type Response struct {
	Selection      int    `json:"selection"`
	Selected       string `json:"selected,omitempty"`
	Elaboration    string `json:"elaboration,omitempty"`
	NoneOfTheAbove bool   `json:"none_of_the_above"`
	Canceled       bool   `json:"canceled"`
}

func ParseResponse(content string, options []string) (*Response, error) {
	numOptions := len(options)

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, fmt.Errorf("enter a number 1-%d or 'none', with optional elaboration following ':'", numOptions)
	}

	selectionToken := trimmed
	response := ""

	before, after, ok := strings.Cut(trimmed, ":")
	if ok {
		selectionToken = strings.TrimSpace(before)
		response = strings.TrimSpace(after)
	}

	if strings.EqualFold(selectionToken, "none") {
		return &Response{Elaboration: response, NoneOfTheAbove: true}, nil
	}

	n, err := strconv.Atoi(selectionToken)
	if err != nil || n < 1 || n > numOptions {
		return nil, fmt.Errorf("enter a number 1-%d or 'none', with optional elaboration following ':'", numOptions)
	}

	return &Response{Selection: n, Selected: options[n-1], Elaboration: response}, nil
}

func (r Response) String() string {
	var str string

	switch {
	case r.Canceled:
		str = "Canceled"
	case r.NoneOfTheAbove:
		str = "None of the above"
		if r.Elaboration != "" {
			str += ": " + r.Elaboration
		}
	default:
		str = r.Selected
	}

	return str
}
