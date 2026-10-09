package domain

type Settings struct {
	Title      string     `json:"title"`
	Subtitle   string     `json:"subtitle"`
	Theme      string     `json:"theme"`
	Accent     string     `json:"accent"`
	Compact    bool       `json:"compact"`
	CustomCSS  string     `json:"customCss"`
	Appearance Appearance `json:"appearance"`
	Widgets    Widgets    `json:"widgets"`
	Revision   int64      `json:"revision"`
}

type SettingsPatch struct {
	Title      *string     `json:"title" binding:"omitempty,max=80"`
	Subtitle   *string     `json:"subtitle" binding:"omitempty,max=200"`
	Theme      *string     `json:"theme" binding:"omitempty,oneof=light dark system"`
	Accent     *string     `json:"accent" binding:"omitempty,oneof=blue green amber slate violet rose custom"`
	Compact    *bool       `json:"compact"`
	CustomCSS  *string     `json:"customCss"`
	Appearance *Appearance `json:"appearance"`
	Widgets    *Widgets    `json:"widgets"`
}

// Appearance and Widgets are replaced as a whole, so false and zero values
// have the same meaning in the editor, API and persisted configuration.
type Appearance struct {
	Layout            string `json:"layout"`
	Columns           int    `json:"columns"`
	Width             string `json:"width"`
	CardStyle         string `json:"cardStyle"`
	IconSize          string `json:"iconSize"`
	Radius            string `json:"radius"`
	FontSize          string `json:"fontSize"`
	FontFamily        string `json:"fontFamily"`
	Background        string `json:"background"`
	BackgroundImage   string `json:"backgroundImage"`
	BackgroundDim     int    `json:"backgroundDim"`
	AccentColor       string `json:"accentColor"`
	ShowTitle         bool   `json:"showTitle"`
	ShowClock         bool   `json:"showClock"`
	ShowSearch        bool   `json:"showSearch"`
	ShowIcons         bool   `json:"showIcons"`
	ShowDescriptions  bool   `json:"showDescriptions"`
	ShowURLs          bool   `json:"showUrls"`
	ShowWebStatus     bool   `json:"showWebStatus"`
	ShowGroupCounts   bool   `json:"showGroupCounts"`
	ShowFooter        bool   `json:"showFooter"`
	CollapsibleGroups bool   `json:"collapsibleGroups"`
}

type Widgets struct {
	ShowHost       bool     `json:"showHost"`
	ShowDocker     bool     `json:"showDocker"`
	RefreshSeconds int      `json:"refreshSeconds"`
	Order          []string `json:"order"`
}

func DefaultAppearance() Appearance {
	return Appearance{Layout: "grid", Width: "normal", CardStyle: "card", IconSize: "medium", Radius: "soft", FontSize: "normal", FontFamily: "noto", Background: "plain", BackgroundDim: 60, ShowTitle: true, ShowClock: true, ShowSearch: true, ShowIcons: true, ShowDescriptions: true, ShowWebStatus: true, ShowGroupCounts: true, ShowFooter: true}
}

func DefaultWidgets() Widgets {
	return Widgets{ShowHost: true, ShowDocker: true, RefreshSeconds: 30, Order: []string{"host", "docker"}}
}
