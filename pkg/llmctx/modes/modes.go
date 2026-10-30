package modes

import (
	"strings"

	"github.com/cneill/smoke/pkg/utils"
)

type Mode string

const (
	ModeWork      = "work"
	ModePlanning  = "planning"
	ModeReview    = "review"
	ModeSummarize = "summarize"
	ModeRanking   = "ranking"
	ModeOneShot   = "oneshot"
)

type Modes []Mode

func (m Modes) String() string {
	return strings.Join(utils.ToStrings(m...), ", ")
}

func AllModes() Modes {
	return []Mode{
		ModeWork,
		ModePlanning,
		ModeReview,
		ModeSummarize,
		ModeRanking,
		ModeOneShot,
	}
}

func SelectableModes() Modes {
	return []Mode{
		ModeWork,
		ModePlanning,
		ModeReview,
	}
}

func DefaultMode() Mode { return ModeWork }
