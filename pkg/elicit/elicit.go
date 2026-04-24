package elicit

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type Request struct {
	Question string
	Options  []string
}

func (r Request) Validate() error {
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

type Submission struct {
	Selection int
	Response  string
}

type Result struct {
	Selection      int    `json:"selection"`
	Selected       string `json:"selected,omitempty"`
	Response       string `json:"response,omitempty"`
	NoneOfTheAbove bool   `json:"none_of_the_above"`
	Canceled       bool   `json:"canceled"`
}

func ParseSubmission(content string, optionCount int) (*Submission, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, fmt.Errorf("enter a number 1-%d or 'none'", optionCount)
	}

	selectionToken := trimmed
	response := ""

	before, after, ok := strings.Cut(trimmed, ":")
	if ok {
		selectionToken = before
		response = after
	} else {
		fields := strings.Fields(trimmed)
		if len(fields) > 1 {
			selectionToken = fields[0]
			response = strings.Join(fields[1:], " ")
		}
	}

	selectionToken = strings.TrimSpace(selectionToken)
	response = strings.TrimSpace(response)

	if strings.EqualFold(selectionToken, "none") {
		return &Submission{Selection: 0, Response: response}, nil
	}

	n, err := strconv.Atoi(selectionToken)
	if err != nil || n < 1 || n > optionCount {
		return nil, fmt.Errorf("enter a number 1-%d or 'none'", optionCount)
	}

	return &Submission{Selection: n, Response: response}, nil
}

func BuildResult(req Request, submission *Submission) Result {
	result := Result{
		Selection: submission.Selection,
		Response:  submission.Response,
	}
	if submission.Selection == 0 {
		result.NoneOfTheAbove = true
	} else {
		result.Selected = req.Options[submission.Selection-1]
	}

	return result
}

type Runtime struct {
	mu      sync.Mutex
	pending *pendingRequest
	onBegin func(Request)
}

type pendingRequest struct {
	request Request
	result  chan Result
}

func NewRuntime() *Runtime {
	return &Runtime{}
}

func (r *Runtime) SetOnBegin(fn func(Request)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.onBegin = fn
}

func (r *Runtime) Begin(req Request) (<-chan Result, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.pending != nil {
		return nil, fmt.Errorf("elicit request already pending")
	}

	pending := &pendingRequest{
		request: req,
		result:  make(chan Result, 1),
	}
	r.pending = pending

	if r.onBegin != nil {
		r.onBegin(req)
	}

	return pending.result, nil
}

func (r *Runtime) ActiveRequest() (Request, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.pending == nil {
		return Request{}, false
	}

	return r.pending.request, true
}

func (r *Runtime) Complete(submission *Submission) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.pending == nil {
		return fmt.Errorf("no active elicit request")
	}

	result := BuildResult(r.pending.request, submission)
	r.pending.result <- result

	r.pending = nil

	return nil
}

func (r *Runtime) Cancel() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.pending == nil {
		return fmt.Errorf("no active elicit request")
	}

	r.pending.result <- Result{Canceled: true}

	r.pending = nil

	return nil
}
