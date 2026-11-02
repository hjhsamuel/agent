package local

// Manifest is the public tool.json format, independent of the model SDK.
type Manifest struct {
	Name                 string         `json:"name"`
	Description          string         `json:"description"`
	ParametersJsonSchema map[string]any `json:"parametersJsonSchema,omitempty"`
	ResponseJsonSchema   map[string]any `json:"responseJsonSchema,omitempty"`
}
