package scheduling

// LLMRequest is a structured representation of the fields we parse out of the LLMRequest body.
type LLMRequest struct {
	Model string
	// Target models is a map of target model name to weight.
	TargetModels map[string]int
	// Resolved target model is the final target model after traffic split.
	ResolvedTargetModel string
	Critical            bool
}
