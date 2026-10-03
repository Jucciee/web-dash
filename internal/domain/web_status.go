package domain

import "time"

// WebStatus is a live observation from the Dashboard server, independent of a
// card's optimistic edit revision. It is intentionally not persisted in SQLite.
type WebStatus struct {
	ServiceID  string     `json:"serviceId"`
	State      string     `json:"state"`
	Checking   bool       `json:"checking"`
	HTTPStatus int        `json:"httpStatus"`
	LatencyMS  int64      `json:"latencyMs"`
	CheckedAt  *time.Time `json:"checkedAt"`
	Message    string     `json:"message"`
}
