package domain

type Settings struct {
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle"`
	Theme     string `json:"theme"`
	Accent    string `json:"accent"`
	Compact   bool   `json:"compact"`
	CustomCSS string `json:"customCss"`
	Revision  int64  `json:"revision"`
}

type SettingsPatch struct {
	Title     *string `json:"title" binding:"omitempty,max=80"`
	Subtitle  *string `json:"subtitle" binding:"omitempty,max=200"`
	Theme     *string `json:"theme" binding:"omitempty,oneof=light dark system"`
	Accent    *string `json:"accent" binding:"omitempty,oneof=blue green amber slate"`
	Compact   *bool   `json:"compact"`
	CustomCSS *string `json:"customCss"`
}
