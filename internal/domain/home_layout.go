package domain

// The home layout only references existing cards. Credentials and API request
// definitions stay in their own records. Auto mode preserves the original home.
type HomeLayout struct {
	Version  int           `json:"version"`
	Mode     string        `json:"mode"`
	Sections []HomeSection `json:"sections"`
}

type HomeSection struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	X         *int       `json:"x,omitempty"`
	Y         *int       `json:"y,omitempty"`
	Width     int        `json:"width"`
	MinRows   int        `json:"minRows"`
	Collapsed bool       `json:"collapsed"`
	Items     []HomeItem `json:"items"`
}

// Both sections and cards use twelve columns. Missing section coordinates keep
// older layouts compatible; the editor places them before the next save.
// A row is 48px, including one extra row for each section's heading;
// gutters are supplied by the UI. Mobile flow leaves desktop coordinates intact.
type HomeItem struct {
	Kind   string `json:"kind"`
	RefID  string `json:"refId"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func DefaultHomeLayout() HomeLayout {
	return HomeLayout{Version: 1, Mode: "auto", Sections: []HomeSection{}}
}
