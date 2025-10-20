// Package rank contains a prompt command to rank arbitrary large lists through iteration. It is based on the raink
// algorithm described and implemented here:
// https://bishopfox.com/blog/raink-llms-document-ranking
// https://github.com/noperator/raink
package rank

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/utils"
)

/*
If the RequestMessage contains a mechanism for LLM messages to propagate back to the Rank command for processing, it can
handle the parsing, retries, etc. by sending more requests. This likely requires control of batching - don't want to be
stitching batches back together from raw LLM messages to try to signal to Smoke what it needs to retry. It's better if
each request is independent, rather than having Smoke manage that in a somewhat opaque way and then try to communicate
back to Rank.
*/

const (
	Name               = "rank"
	MultilineSeparator = "\n----\n"
)

type RequestMessage struct {
	commands.MessageType

	PromptMessage commands.PromptMessage
	Iteration     int
	BatchIdx      int
	Batch         Items
	Description   string
	ResponseChan  chan<- ResponseMessage
	Retries       int
}

type ResponseMessage struct {
	RequestMessage

	Message string
	Err     error
}

type Rank struct {
	PromptMessage commands.PromptMessage
	listPath      string
	listContents  string
	description   string
	allItems      Items

	teaEmitter uimsg.TeaEmitter
}

func New(msg commands.PromptMessage) (commands.Command, error) {
	// Handle help generation separately
	if len(msg.Args) == 1 && msg.Args[0] == "help" {
		return &Rank{PromptMessage: msg}, nil
	}

	handler := &Rank{
		PromptMessage: msg,
	}

	if len(msg.Args) < 2 {
		return nil, fmt.Errorf("%w: usage: %s", commands.ErrArguments, handler.Usage())
	}

	handler.listPath = msg.Args[0]
	handler.description = strings.Join(msg.Args[1:], " ")

	contents, err := os.ReadFile(handler.listPath)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read contents of list file %q: %w", commands.ErrArguments, handler.listPath, err)
	}

	handler.listContents = string(contents)

	items, err := handler.splitItems(handler.listContents)
	if err != nil {
		return nil, err
	}

	handler.allItems = items

	return handler, nil
}

func (r *Rank) Name() string { return Name }

func (r *Rank) Help() string {
	return "Asks the LLM to rank a large, arbitrary list based on the user's criteria."
}

func (r *Rank) Usage() string {
	return "/rank <list_file> <description>"
}

func (r *Rank) SetTeaEmitter(emitter uimsg.TeaEmitter) {
	r.teaEmitter = emitter
}

func (r *Rank) Run(_ *llms.Session) (tea.Cmd, error) {
	go r.looper(r.allItems)

	msg := commands.HistoryUpdateMessage{
		PromptMessage: r.PromptMessage,
		Message:       fmt.Sprintf("Starting requested ranking of %d items...", len(r.allItems)),
	}

	return uimsg.MsgToCmd(msg), nil
}

func (r *Rank) splitItems(contents string) (Items, error) {
	var rawItems []string

	// TODO: JSON?
	if strings.Contains(contents, MultilineSeparator) {
		// we have potential multi-line list items
		rawItems = strings.Split(contents, MultilineSeparator)
	} else {
		// we assume we are dealing with a list containing 1 item per line
		rawItems = strings.Split(contents, "\n")
	}

	if n := len(rawItems); n < 2 {
		return nil, fmt.Errorf("only found %d items in list, need at least 2 to rank", n)
	}

	items := make(Items, len(rawItems))

	for i, rawItem := range rawItems {
		items[i] = &Item{
			ID:          utils.RandID(8),
			Contents:    rawItem,
			RankHistory: []int{},
		}
	}

	return items, nil
}

// TODO: better name
func (r *Rank) looper(items Items) {
	numIterations := 5
	batchSize := 10

	batches, err := r.allItems.Batch(batchSize)
	if err != nil {
		msg := uimsg.ToError(fmt.Errorf("failed to create batches of items: %w", err))
		r.teaEmitter(msg)

		return
	}

	responseChan := make(chan ResponseMessage)
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*90)
	defer cancel()

	wg.Go(func() {
		r.responseListener(ctx, len(batches), responseChan)
	})

	// For each batch, we rank-order its items multiple times to make sure we get some consistency/stability.
	for iteration := range numIterations {
		for batchIdx, batch := range batches {
			slog.Debug("requesting ranking", "iteration", iteration, "batch_index", batchIdx, "batch", batch)

			msg := RequestMessage{
				PromptMessage: r.PromptMessage,
				Iteration:     iteration,
				BatchIdx:      batchIdx,
				Batch:         batch,
				Description:   r.description,
				ResponseChan:  responseChan,
				Retries:       3, // TODO: do something with this
			}

			r.teaEmitter(msg)
		}
	}

	wg.Wait()

	// We now take the results, filter to the top X%, and run this whole process over again with the filtered items.
}

func (r *Rank) responseListener(ctx context.Context, numBatches int, responseChan <-chan ResponseMessage) {
	failures := 0
	successes := 0
	results := make([]Items, numBatches)

listenLoop:
	for {
		select {
		// TODO: context / cancellation?
		case <-ctx.Done():
			slog.Error("context finished before completion", "error", ctx.Err())
			break listenLoop
		case response, ok := <-responseChan:
			if !ok {
				break listenLoop
			}

			if response.Err != nil {
				// TODO: retry
				slog.Error("got error in ranking response listener", "error", response.Err, "request", response.RequestMessage)

				failures++

				continue
			}

			result := []string{}
			if err := json.Unmarshal([]byte(response.Message), &result); err != nil {
				// TODO: retry
				slog.Error("failed to parse assistant response as JSON string list", "error", err, "request", response.RequestMessage)

				failures++

				continue
			}

			slog.Debug("got rankings", "batch_idx", response.BatchIdx, "iteration", response.Iteration, "rankings", result)

			if err := response.Batch.AddRankings(result); err != nil {
				// TODO: retry? what do?
				slog.Error("failed to add rankings for batch", "idx", response.BatchIdx, "iteration", response.Iteration, "error", err)

				failures++

				continue
			}

			results[response.BatchIdx] = response.Batch
			successes++

		default:
			if failures >= 5 {
				slog.Error("got 5 or more failures, response listener bailing")
				break listenLoop
			}
		}
	}

	if successes != numBatches {
		slog.Info("failed to get all expected results", "successes", successes, "expected", numBatches)
		return
	}
}
