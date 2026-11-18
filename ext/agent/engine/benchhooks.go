package engine

import (
	"math"

	"github.com/pockethost/pocketone/ext/agent/engine/internal/benchhooks"
)

func init() {
	benchhooks.TeacherForce = func(m any, prompt, cont []int) ([]int, []float64) {
		return m.(*Model).teacherForce(prompt, cont)
	}
	benchhooks.RequantizeQ8_0 = requantizeQ8_0
}

// teacherForce runs prompt then cont through the model and, for each
// position of cont, returns the model's greedy choice and the log-probability
// it gives cont's token there (for comparing against another engine).
func (m *Model) teacherForce(prompt, cont []int) (argmax []int, logprob []float64) {
	s := m.newState(len(prompt)+len(cont)+1, min(len(prompt), prefillBatch))
	for pos := 0; pos < len(prompt); pos += prefillBatch {
		m.forward(s, prompt[pos:min(pos+prefillBatch, len(prompt))], pos)
	}
	for i, tok := range cont {
		best, mx := 0, float32(math.Inf(-1))
		for j, v := range s.logits {
			if v > mx {
				best, mx = j, v
			}
		}
		var sum float64
		for _, v := range s.logits {
			sum += math.Exp(float64(v - mx))
		}
		argmax = append(argmax, best)
		logprob = append(logprob, float64(s.logits[tok]-mx)-math.Log(sum))
		m.forward(s, []int{tok}, len(prompt)+i)
	}
	return argmax, logprob
}
