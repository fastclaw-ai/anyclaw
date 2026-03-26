package config

import (
	"fmt"
	"strings"
)

// openAPISpec is a minimal representation of an OpenAPI 3.x document.
type openAPISpec struct {
	OpenAPI    string                          `yaml:"openapi"`
	Info       openAPIInfo                     `yaml:"info"`
	Servers    []openAPIServer                 `yaml:"servers"`
	Paths      map[string]map[string]openAPIOp `yaml:"paths"`
	Components *openAPIComponents              `yaml:"components"`
}

type openAPIInfo struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
}

type openAPIServer struct {
	URL string `yaml:"url"`
}

type openAPIOp struct {
	OperationID string          `yaml:"operationId"`
	Summary     string          `yaml:"summary"`
	Description string          `yaml:"description"`
	Parameters  []openAPIParam  `yaml:"parameters"`
	RequestBody *openAPIReqBody `yaml:"requestBody"`
}

type openAPIParam struct {
	Name        string        `yaml:"name"`
	In          string        `yaml:"in"` // query, path, header
	Required    bool          `yaml:"required"`
	Description string        `yaml:"description"`
	Schema      openAPISchema `yaml:"schema"`
}

type openAPIReqBody struct {
	Required bool                        `yaml:"required"`
	Content  map[string]openAPIMediaType `yaml:"content"`
}

type openAPIMediaType struct {
	Schema openAPISchema `yaml:"schema"`
}

type openAPISchema struct {
	Type        string                   `yaml:"type"`
	Description string                   `yaml:"description"`
	Properties  map[string]openAPISchema `yaml:"properties"`
	Required    []string                 `yaml:"required"`
	Default     any                      `yaml:"default"`
	Ref         string                   `yaml:"$ref"`
}

type openAPIComponents struct {
	Schemas         map[string]openAPISchema        `yaml:"schemas"`
	SecuritySchemes map[string]openAPISecurityScheme `yaml:"securitySchemes"`
}

type openAPISecurityScheme struct {
	Type   string `yaml:"type"`   // apiKey, http
	Scheme string `yaml:"scheme"` // bearer, basic
	Name   string `yaml:"name"`   // header name for apiKey
	In     string `yaml:"in"`     // header, query
}

// fromOpenAPI converts an OpenAPI spec into an anyclaw Config.
func fromOpenAPI(spec *openAPISpec) *Config {
	cfg := &Config{
		Name:        sanitizeName(spec.Info.Title),
		Description: spec.Info.Description,
		Backend: Backend{
			Type: "http",
		},
	}

	// Use first server as base URL
	if len(spec.Servers) > 0 {
		cfg.Backend.BaseURL = strings.TrimRight(spec.Servers[0].URL, "/")
	}

	// Convert security schemes to auth config
	if spec.Components != nil {
		for _, scheme := range spec.Components.SecuritySchemes {
			cfg.Backend.Auth = convertSecurityScheme(scheme)
			break // use first one
		}
	}

	// Convert paths to skills
	for path, methods := range spec.Paths {
		for method, op := range methods {
			skill := convertOperation(path, method, op, spec)
			cfg.Skills = append(cfg.Skills, skill)
		}
	}

	return cfg
}

func convertOperation(path, method string, op openAPIOp, spec *openAPISpec) Skill {
	name := op.OperationID
	if name == "" {
		// Generate name from method + path: GET /users/{id} -> get_users_by_id
		cleaned := strings.ReplaceAll(path, "{", "")
		cleaned = strings.ReplaceAll(cleaned, "}", "")
		cleaned = strings.ReplaceAll(cleaned, "/", "_")
		cleaned = strings.Trim(cleaned, "_")
		name = strings.ToLower(method) + "_" + cleaned
	}

	desc := op.Summary
	if desc == "" {
		desc = op.Description
	}

	skill := Skill{
		Name:        name,
		Description: desc,
		Input:       make(map[string]Field),
		Backend: SkillBackend{
			Method: strings.ToUpper(method),
			Path:   path,
		},
	}

	// Convert parameters — skip path params (they are part of the URL path)
	for _, param := range op.Parameters {
		if param.In == "path" {
			// Path params are embedded in the URL, not sent as query/body params.
			// Store them as input so the caller knows they're needed,
			// and the HTTP client will substitute {name} in the path.
			skill.Input[param.Name] = Field{
				Type:        schemaType(param.Schema),
				Required:    true, // path params are always required
				Description: param.Description,
			}
			continue
		}
		field := Field{
			Type:        schemaType(param.Schema),
			Required:    param.Required,
			Description: param.Description,
		}
		if param.Schema.Default != nil {
			field.Default = fmt.Sprintf("%v", param.Schema.Default)
		}
		skill.Input[param.Name] = field
	}

	// Convert request body properties
	if op.RequestBody != nil {
		for _, mediaType := range op.RequestBody.Content {
			schema := resolveSchema(mediaType.Schema, spec)
			requiredSet := make(map[string]bool)
			for _, r := range schema.Required {
				requiredSet[r] = true
			}
			for propName, propSchema := range schema.Properties {
				resolved := resolveSchema(propSchema, spec)
				field := Field{
					Type:        schemaType(resolved),
					Required:    requiredSet[propName],
					Description: resolved.Description,
				}
				if resolved.Default != nil {
					field.Default = fmt.Sprintf("%v", resolved.Default)
				}
				skill.Input[propName] = field
			}
			break // use first content type
		}
	}

	return skill
}

func convertSecurityScheme(scheme openAPISecurityScheme) *Auth {
	switch scheme.Type {
	case "http":
		switch scheme.Scheme {
		case "bearer":
			return &Auth{Type: "bearer", TokenEnv: "API_TOKEN"}
		case "basic":
			return &Auth{Type: "basic", TokenEnv: "API_TOKEN"}
		}
	case "apiKey":
		return &Auth{Type: "api_key", Header: scheme.Name, TokenEnv: "API_KEY"}
	}
	return nil
}

func resolveSchema(schema openAPISchema, spec *openAPISpec) openAPISchema {
	if schema.Ref == "" || spec.Components == nil {
		return schema
	}
	ref := schema.Ref
	const prefix = "#/components/schemas/"
	if strings.HasPrefix(ref, prefix) {
		name := ref[len(prefix):]
		if resolved, ok := spec.Components.Schemas[name]; ok {
			return resolved
		}
	}
	return schema
}

func schemaType(schema openAPISchema) string {
	if schema.Type != "" {
		return schema.Type
	}
	return "string"
}

func sanitizeName(title string) string {
	name := strings.ToLower(title)
	name = strings.ReplaceAll(name, " ", "-")
	var buf strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			buf.WriteRune(c)
		}
	}
	result := buf.String()
	if result == "" {
		return "api"
	}
	return result
}
