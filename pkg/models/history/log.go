package history

import (
	"slices"
	"sync"

	"github.com/cneill/smoke/pkg/llms"
)

// Log stores the messages rendered as history in the viewport. It handles messages streamed from LLMs and updates them
// as chunks come in.
type Log struct {
	mutex sync.RWMutex
	log   []any
}

func NewLog() *Log {
	return &Log{
		mutex: sync.RWMutex{},
		log:   []any{},
	}
}

func (l *Log) Messages() []any {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	// not sure this is truly necessary...
	return append([]any{}, l.log...)
}

func (l *Log) AddMessage(message any) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// If we have a non-LLM message (e.g. error), add it directly to the log and return.
	llmMessage, ok := message.(*llms.Message)
	if !ok {
		l.log = append(l.log, message)
		return
	}

	// If we don't recognize the ID of this message, append it to the log and return.
	msgIdx := l.llmMessageIndexByID(llmMessage.ID)
	if msgIdx == -1 {
		l.log = append(l.log, message)
		return
	}

	// If this message ID already exists in the log, overwrite it.
	l.log[msgIdx] = llmMessage
}

func (l *Log) RefreshLog(log []any) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.log = log
}

func (l *Log) llmMessageIndexByID(id string) int {
	for logIdx := range slices.Backward(l.log) {
		logMessage, ok := l.log[logIdx].(*llms.Message)
		if !ok {
			continue
		}

		if logMessage.ID == id {
			return logIdx
		}
	}

	return -1
}
