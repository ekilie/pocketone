// Package benchhooks gives cmd/agentbench access to engine internals that are
// not part of the engine's API. Package engine fills these in at init.
package benchhooks

var (
	// TeacherForce runs prompt then cont through m (an *engine.Model) and, for
	// each position of cont, returns the model's greedy choice and the
	// log-probability it gives cont's token there.
	TeacherForce func(m any, prompt, cont []int) (argmax []int, logprob []float64)

	// RequantizeQ8_0 writes a copy of the GGUF file in to out with every 2-D
	// weight converted to Q8_0 (for testing the Q8_0 path).
	RequantizeQ8_0 func(in, out string) error
)
