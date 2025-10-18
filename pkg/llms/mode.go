package llms

type Mode string

const (
	ModeWork      = "work"
	ModePlanning  = "planning"
	ModeReview    = "review"
	ModeSummarize = "summarize"
	ModeRanking   = "ranking"
)

func AllModes() []Mode {
	return []Mode{
		ModeWork,
		ModePlanning,
		ModeReview,
		ModeSummarize,
		ModeRanking,
	}
}

func SelectableModes() []Mode {
	return []Mode{
		ModeWork,
		ModePlanning,
		ModeReview,
	}
}
