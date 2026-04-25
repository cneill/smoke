package input

import (
	"fmt"
	"sync"

	"github.com/cneill/smoke/pkg/elicit"
)

type ElicitState struct {
	Request *elicit.RequestMessage
	mu      sync.RWMutex
}

func newElicitState() *ElicitState {
	return &ElicitState{}
}

func (e *ElicitState) newRequest(request elicit.RequestMessage) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.Request != nil {
		return fmt.Errorf("rejecting elicit request while one is already active")
	}

	e.Request = &request

	return nil
}

func (e *ElicitState) isActive() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.Request != nil
}

func (e *ElicitState) currentRequest() *elicit.RequestMessage {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.Request
}

func (e *ElicitState) endRequest() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.Request = nil
}
