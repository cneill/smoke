package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/plan"
)

const (
	PlanAddType           = "type"
	PlanAddTypeTask       = "task"
	PlanAddTypeContext    = "context"
	PlanAddTypeCompletion = "completion"
)

type PlanAddTool struct {
	ProjectPath  string
	SessionName  string
	PlanFileName string
	PlanFile     *os.File
	PlanManager  *plan.Manager
}

// TODO: allow tool initializations to fail with error...?
func NewPlanAddTool(projectPath, sessionName string) Tool {
	planFileName := sessionName + "_plan.json"

	relPath, err := fs.GetRelativePath(projectPath, planFileName)
	if err != nil {
		panic(fmt.Errorf("invalid session plan file path (%s): %w", planFileName, err))
	}

	// TODO: stat for existing plan file, create the manager by loading if exists
	file, err := os.OpenFile(relPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		panic(fmt.Errorf("failed to open session plan file: %w", err))
	}

	manager := plan.NewManager(file)

	return &PlanAddTool{
		ProjectPath:  projectPath,
		SessionName:  sessionName,
		PlanFileName: planFileName,
		PlanManager:  manager,
	}
}

func (p *PlanAddTool) Name() string { return ToolPlanAdd }
func (p *PlanAddTool) Description() string {
	examples := CollectExamples(p.Examples()...)

	// TODO
	return "Blah:  " + p.PlanFileName + examples
}

func (p *PlanAddTool) Examples() Examples {
	return Examples{
		// TODO
	}
}

func (p *PlanAddTool) Params() Params {
	return Params{
		{
			Key:              PlanAddType,
			Description:      "The type of item to add to the plan",
			Type:             ParamTypeString,
			Required:         true,
			EnumStringValues: []string{PlanAddTypeTask, PlanAddTypeContext, PlanAddTypeCompletion},
		},
	}
}

func (p *PlanAddTool) Run(_ context.Context, args Args) (string, error) {
	return "", nil
}
