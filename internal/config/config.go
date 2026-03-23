package config

// Config is the top-level anyclaw configuration.
type Config struct {
	Name        string  `mapstructure:"name"`
	Description string  `mapstructure:"description"`
	Backend     Backend `mapstructure:"backend"`
	Skills      []Skill `mapstructure:"skills"`
}

// Backend describes how to reach the target service.
type Backend struct {
	Type    string `mapstructure:"type"`     // "http", "command", "mcp"
	BaseURL string `mapstructure:"base_url"` // for http type
	Auth    *Auth  `mapstructure:"auth"`
}

// Auth describes authentication for the backend.
type Auth struct {
	Type     string `mapstructure:"type"`      // "bearer", "basic", "api_key"
	TokenEnv string `mapstructure:"token_env"` // env var name holding the token
	Header   string `mapstructure:"header"`    // custom header name (for api_key type)
}

// Skill describes a single capability exposed by the agent.
type Skill struct {
	Name        string           `mapstructure:"name"`
	Description string           `mapstructure:"description"`
	Input       map[string]Field `mapstructure:"input"`
	Output      map[string]Field `mapstructure:"output"`
	Backend     SkillBackend     `mapstructure:"backend"`
}

// Field describes a single input or output parameter.
type Field struct {
	Type        string `mapstructure:"type"`
	Required    bool   `mapstructure:"required"`
	Description string `mapstructure:"description"`
	Default     string `mapstructure:"default"`
}

// SkillBackend describes how a specific skill maps to the backend.
type SkillBackend struct {
	Method string `mapstructure:"method"` // HTTP method: GET, POST, etc.
	Path   string `mapstructure:"path"`   // URL path appended to base_url
}
