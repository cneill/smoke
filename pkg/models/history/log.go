package history

import (
	"log/slog"
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

	// If we have:
	// - a non-LLM message (e.g. error)
	// - an LLM message that wasn't streamed (thus inherently final)
	// - a streamed LLM message that is marked as "initial" (with other chunks with the same ID to come)
	//
	// Add it directly to the log and return.
	llmMessage, ok := message.(*llms.Message)
	if !ok || !llmMessage.IsStreamed || llmMessage.IsInitial {
		l.log = append(l.log, message)
		slog.Debug("added message to log", "message", message)

		return
	}

	msgIdx, logMsg := l.llmMessageIndexByID(llmMessage.ID)
	if msgIdx == -1 {
		slog.Error("got a streamed LLM message we couldn't account for", "message", message)
		return
	}

	if logMsg.IsFinalized {
		slog.Error("tried to modify finalized log message", "message", logMsg)
		return
	}

	if llmMessage.IsFinalized {
		l.log[msgIdx] = llmMessage
	} else {
		l.log[msgIdx] = logMsg.Update(llms.WithContent(llmMessage.Content))
	}
}

func (l *Log) RefreshLog(log []any) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.log = log
}

func (l *Log) llmMessageIndexByID(id string) (int, *llms.Message) {
	for logIdx := len(l.log) - 1; logIdx >= 0; logIdx-- {
		logMessage, ok := l.log[logIdx].(*llms.Message)
		if !ok {
			continue
		}

		if logMessage.ID == id {
			return logIdx, logMessage
		}
	}

	return -1, nil
}
