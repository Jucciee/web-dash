package domain

import "time"

// A manifest is portable configuration. Instance values and credentials are
// separate, so sharing a manifest never includes configured secret inputs.
type AddonManifest struct {
	SchemaVersion  int          `json:"schemaVersion"`
	Name           string       `json:"name"`
	Icon           string       `json:"icon"`
	Display        string       `json:"display"`
	RefreshSeconds int          `json:"refreshSeconds"`
	Request        AddonRequest `json:"request"`
	Inputs         []AddonInput `json:"inputs"`
	Fields         []AddonField `json:"fields"`
}

type AddonRequest struct {
	Method  string        `json:"method"`
	URL     string        `json:"url"`
	Headers []AddonHeader `json:"headers"`
	Body    string        `json:"body"`
}

type AddonHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type AddonInput struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Default  string `json:"default"`
}
type AddonField struct {
	Label    string            `json:"label"`
	Path     string            `json:"path"`
	Format   string            `json:"format"`
	Unit     string            `json:"unit"`
	Scale    float64           `json:"scale"`
	Decimals int               `json:"decimals"`
	ValueMap map[string]string `json:"valueMap"`
}
type Addon struct {
	ID                string            `json:"id"`
	Manifest          AddonManifest     `json:"manifest"`
	Values            map[string]string `json:"values"`
	ConfiguredSecrets []string          `json:"configuredSecrets"`
	Enabled           bool              `json:"enabled"`
	Position          int               `json:"position"`
	Revision          int64             `json:"revision"`
}
type AddonWrite struct {
	Manifest AddonManifest     `json:"manifest"`
	Values   map[string]string `json:"values"`
	Secrets  map[string]string `json:"secrets"`
	Enabled  bool              `json:"enabled"`
}
type AddonPreview struct {
	AddonWrite
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}
type AddonValue struct {
	Label   string `json:"label"`
	Value   string `json:"value"`
	Unit    string `json:"unit"`
	Missing bool   `json:"missing"`
}
type AddonChoice struct {
	Path  string `json:"path"`
	Value string `json:"value"`
	Type  string `json:"type"`
}
type AddonSnapshot struct {
	Available  bool          `json:"available"`
	HTTPStatus int           `json:"httpStatus"`
	LatencyMS  int64         `json:"latencyMs"`
	MeasuredAt time.Time     `json:"measuredAt"`
	Message    string        `json:"message"`
	Values     []AddonValue  `json:"values"`
	Choices    []AddonChoice `json:"choices"`
}
