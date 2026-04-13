package handlers

import (
	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/tools/handlers/ddg"
	"github.com/cneill/smoke/pkg/tools/handlers/edit"
	gitdiff "github.com/cneill/smoke/pkg/tools/handlers/git/diff"
	"github.com/cneill/smoke/pkg/tools/handlers/gofumpt"
	"github.com/cneill/smoke/pkg/tools/handlers/goimports"
	"github.com/cneill/smoke/pkg/tools/handlers/golint"
	"github.com/cneill/smoke/pkg/tools/handlers/gotest"
	"github.com/cneill/smoke/pkg/tools/handlers/grep"
	"github.com/cneill/smoke/pkg/tools/handlers/listfiles"
	"github.com/cneill/smoke/pkg/tools/handlers/mkdir"
	planadd "github.com/cneill/smoke/pkg/tools/handlers/plan/add"
	plancompletion "github.com/cneill/smoke/pkg/tools/handlers/plan/completion"
	planread "github.com/cneill/smoke/pkg/tools/handlers/plan/read"
	planupdate "github.com/cneill/smoke/pkg/tools/handlers/plan/update"
	"github.com/cneill/smoke/pkg/tools/handlers/readfile"
	skillshandler "github.com/cneill/smoke/pkg/tools/handlers/skills"
	"github.com/cneill/smoke/pkg/tools/handlers/writefile"
)

func AllTools() []tools.Initializer {
	return []tools.Initializer{
		ddg.New,
		edit.New,
		gitdiff.New,
		gofumpt.New,
		goimports.New,
		golint.New,
		gotest.New,
		grep.New,
		listfiles.New,
		mkdir.New,
		planadd.New,
		plancompletion.New,
		planread.New,
		planupdate.New,
		// TODO: figure out how to actually support images w/ multimodal models
		// playwright.New,
		readfile.New,
		skillshandler.New,
		// replacelines.New,
		// summarize.New,
		writefile.New,
	}
}

func WorkTools() []tools.Initializer {
	return AllTools()
}

func PlanningTools() []tools.Initializer {
	return []tools.Initializer{
		ddg.New,
		gitdiff.New,
		golint.New,
		gotest.New,
		grep.New,
		listfiles.New,
		planadd.New,
		plancompletion.New,
		planread.New,
		planupdate.New,
		readfile.New,
		skillshandler.New,
	}
}

func ReviewTools() []tools.Initializer {
	return []tools.Initializer{
		ddg.New,
		gitdiff.New,
		golint.New,
		gotest.New,
		grep.New,
		listfiles.New,
		planadd.New,
		plancompletion.New, // TODO: ?
		planread.New,
		planupdate.New,
		readfile.New,
		skillshandler.New,
	}
}

func SummarizeTools() []tools.Initializer {
	return []tools.Initializer{
		planread.New,
	}
}

func RankingTools() []tools.Initializer {
	return []tools.Initializer{}
}
