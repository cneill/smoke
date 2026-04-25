package elicit

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type Response struct {
	Selection      int    `json:"selection"`
	Selected       string `json:"selected,omitempty"`
	Elaboration    string `json:"elaboration,omitempty"`
	NoneOfTheAbove bool   `json:"none_of_the_above"`
	Canceled       bool   `json:"canceled"`
}

type Manager struct {
	mu      sync.RWMutex
	pending *pendingRequest
	onBegin func(RequestMessage)
}

type pendingRequest struct {
	request      RequestMessage
	responseChan chan *Response
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) SetOnBegin(fn func(RequestMessage)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onBegin = fn
}

func (m *Manager) Begin(question string, options []string) (<-chan *Response, error) {
	req := RequestMessage{
		Question: question,
		Options:  options,
	}
	if err := req.OK(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pending != nil {
		return nil, fmt.Errorf("elicit request already pending")
	}

	pending := &pendingRequest{
		request:      req,
		responseChan: make(chan *Response, 1),
	}
	m.pending = pending

	if m.onBegin != nil {
		m.onBegin(req)
	}

	return pending.responseChan, nil
}

func (m *Manager) ActiveRequest() (RequestMessage, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.pending == nil {
		return RequestMessage{}, false
	}

	return m.pending.request, true
}

func (m *Manager) ParseUserInput(msg UserInputMessage) (*Response, error) {
	request, ok := m.ActiveRequest()
	if !ok {
		return nil, fmt.Errorf("no active elicit request to answer")
	}

	numOptions := len(request.Options)

	trimmed := strings.TrimSpace(msg.Content)
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

	return &Response{Selection: n, Selected: request.Options[n-1], Elaboration: response}, nil
}

func (m *Manager) Complete(response *Response) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pending == nil {
		return fmt.Errorf("no active elicit request")
	}

	m.pending.responseChan <- response

	m.pending = nil

	return nil
}

func (m *Manager) Cancel() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pending == nil {
		return fmt.Errorf("no active elicit request")
	}

	m.pending.responseChan <- &Response{Canceled: true}

	m.pending = nil

	return nil
}
