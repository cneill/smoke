package handlers

import (
	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/tools/handlers/ddg"
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
	"github.com/cneill/smoke/pkg/tools/handlers/replacelines"
	"github.com/cneill/smoke/pkg/tools/handlers/writefile"
)

func AllTools() []tools.Initializer {
	return []tools.Initializer{
		ddg.New,
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
		readfile.New,
		replacelines.New,
		// summarize.New,
		writefile.New,
	}
}

func NormalTools() []tools.Initializer {
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
	}
}

func SummarizeTools() []tools.Initializer {
	return []tools.Initializer{
		planread.New,
	}
}
