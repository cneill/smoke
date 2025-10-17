package llms

type Mode string

const (
	ModeWork      = "work"
	ModePlanning  = "planning"
	ModeReview    = "review"
	ModeSummarize = "summarize"
)

func AllModes() []Mode {
	return []Mode{
		ModeWork,
		ModePlanning,
		ModeReview,
		ModeSummarize,
	}
}

func SelectableModes() []Mode {
	return []Mode{
		ModeWork,
		ModePlanning,
		ModeReview,
	}
}
