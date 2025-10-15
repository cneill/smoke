package handlers

import (
	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/commands/handlers/edit"
	"github.com/cneill/smoke/pkg/commands/handlers/exit"
	"github.com/cneill/smoke/pkg/commands/handlers/export"
	"github.com/cneill/smoke/pkg/commands/handlers/help"
	"github.com/cneill/smoke/pkg/commands/handlers/info"
	"github.com/cneill/smoke/pkg/commands/handlers/load"
	"github.com/cneill/smoke/pkg/commands/handlers/plan"
	"github.com/cneill/smoke/pkg/commands/handlers/review"
	"github.com/cneill/smoke/pkg/commands/handlers/run"
	"github.com/cneill/smoke/pkg/commands/handlers/save"
	"github.com/cneill/smoke/pkg/commands/handlers/session"
	"github.com/cneill/smoke/pkg/commands/handlers/summarize"
)

func AllCommands() map[string]commands.Initializer {
	initializers := map[string]commands.Initializer{
		edit.Name:      edit.New,
		exit.Name:      exit.New,
		export.Name:    export.New,
		info.Name:      info.New,
		load.Name:      load.New,
		plan.Name:      plan.New,
		review.Name:    review.New,
		run.Name:       run.New,
		save.Name:      save.New,
		session.Name:   session.New,
		summarize.Name: summarize.New,
	}

	// NOTE: since the map gets updated, help ultimately contains itself
	helpCmd := help.New(initializers)
	initializers[help.Name] = helpCmd

	return initializers
}
