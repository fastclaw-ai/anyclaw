package config

// Config is the top-level anyclaw configuration (internal representation).
type Config struct {
	Name        string
	Description string
	Backend     Backend
	Skills      []Skill
}

// Backend describes how to reach the target service.
type Backend struct {
	Type    string // "http"
	BaseURL string
	Auth    *Auth
}

// Auth describes authentication for the backend.
type Auth struct {
	Type     string // "bearer", "basic", "api_key"
	TokenEnv string // env var name holding the token
	Header   string // custom header name (for api_key type)
}

// Skill describes a single capability exposed by the agent.
type Skill struct {
	Name        string
	Description string
	Input       map[string]Field
	Output      map[string]Field
	Backend     SkillBackend
}

// Field describes a single input or output parameter.
type Field struct {
	Type        string
	Required    bool
	Description string
	Default     string
}

// SkillBackend describes how a specific skill maps to the backend.
type SkillBackend struct {
	Method   string         // HTTP method: GET, POST, etc.
	Path     string         // URL path appended to base_url
	Response *SkillResponse // optional response extraction
}

// SkillResponse describes how to extract and format the backend response.
type SkillResponse struct {
	Field    string // dot-separated JSON path to extract
	Template string // optional Go template, {{.value}} is the extracted field
}
