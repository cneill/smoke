// Package rank contains a prompt command to rank arbitrary large lists through iteration. It is based on the raink
// algorithm described and implemented here:
// https://bishopfox.com/blog/raink-llms-document-ranking
// https://github.com/noperator/raink
package rank

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cneill/smoke/internal/uimsg"
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/utils"
)

const (
	Name               = "rank"
	MultilineSeparator = "\n----\n"
)

type RequestMessage struct {
	commands.MessageType

	PromptMessage commands.PromptMessage
	Items         Items
	Description   string
}

type Rank struct {
	PromptMessage commands.PromptMessage
	listPath      string
	listContents  string
	description   string
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

	return handler, nil
}

func (r *Rank) Name() string { return Name }

func (r *Rank) Help() string {
	return "Asks the LLM to rank a large, arbitrary list based on the user's criteria."
}

func (r *Rank) Usage() string {
	return "/rank <list_file> <description>"
}

func (r *Rank) Run(_ *llms.Session) (tea.Cmd, error) {
	items, err := r.splitItems()
	if err != nil {
		return nil, err
	}

	msg := RequestMessage{
		PromptMessage: r.PromptMessage,
		Items:         items,
		Description:   r.description,
	}

	return uimsg.MsgToCmd(msg), nil
}

func (r *Rank) splitItems() (Items, error) {
	var rawItems []string

	// TODO: JSON?
	if strings.Contains(r.listContents, MultilineSeparator) {
		// we have potential multi-line list items
		rawItems = strings.Split(r.listContents, MultilineSeparator)
	} else {
		// we assume we are dealing with a list containing 1 item per line
		rawItems = strings.Split(r.listContents, "\n")
	}

	if n := len(rawItems); n < 2 {
		return nil, fmt.Errorf("only found %d items in list, need at least 2 to rank", n)
	}

	items := make(Items, len(rawItems))

	for i, rawItem := range rawItems {
		items[i] = &Item{
			ID:       utils.RandID(8),
			Contents: rawItem,
			History:  []int{},
		}
	}

	return items, nil
}
