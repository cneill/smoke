package elicit

import (
	"fmt"
	"sync"
)

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

func (m *Manager) Begin(req RequestMessage) (<-chan *Response, error) {
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
