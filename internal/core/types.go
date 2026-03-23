package core

// SkillRequest is the unified request for skill invocation.
type SkillRequest struct {
	SkillName string
	Params    map[string]any
}
