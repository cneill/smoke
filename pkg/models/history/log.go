package history

import (
	"log/slog"
	"sync"

	"github.com/cneill/smoke/pkg/llms"
)

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

	llmMessage, ok := message.(*llms.Message)
	if !ok || !llmMessage.IsStreamed || llmMessage.IsInitial {
		l.log = append(l.log, message)

		return
	}

	found := false
	for i := len(l.log) - 1; i > 0; i-- {
		if item, ok := l.log[i].(*llms.Message); ok {
			if item.ID == llmMessage.ID {
				if llmMessage.IsFinalized {
					l.log[i] = message
				} else {
					item = item.Update(llms.WithContent(llmMessage.Content))
					l.log[i] = item
				}

				found = true

				break
			}
		}
	}

	if !found {
		for i, item := range l.log {
			slog.Debug("LOG ITEM", "num", i, "item", item)
		}

		slog.Error("got a streamed LLM message we couldn't account for", "message", message)
	}
}

func (l *Log) RefreshLog(log []any) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.log = log
}
