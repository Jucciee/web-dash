package storage

import "time"

type UserModel struct {
	ID           string `gorm:"primaryKey;size:36"`
	Singleton    int    `gorm:"not null;uniqueIndex;check:singleton = 1"`
	Username     string `gorm:"not null;uniqueIndex"`
	PasswordHash string `gorm:"not null"`
	CreatedAt    time.Time
}

func (UserModel) TableName() string { return "users" }

type DashboardModel struct {
	ID       string `gorm:"primaryKey;size:36"`
	Name     string `gorm:"not null"`
	Revision int64  `gorm:"not null;default:1"`
}

func (DashboardModel) TableName() string { return "dashboards" }

type GroupModel struct {
	ID          string         `gorm:"primaryKey;size:36"`
	DashboardID string         `gorm:"not null;index"`
	Dashboard   DashboardModel `gorm:"foreignKey:DashboardID;constraint:OnDelete:RESTRICT"`
	Name        string         `gorm:"not null"`
	Position    int            `gorm:"not null;check:position >= 0"`
	Revision    int64          `gorm:"not null;default:1"`
}

func (GroupModel) TableName() string { return "groups" }

type ServiceModel struct {
	ID              string     `gorm:"primaryKey;size:36"`
	GroupID         string     `gorm:"not null;index"`
	Group           GroupModel `gorm:"foreignKey:GroupID;constraint:OnDelete:RESTRICT"`
	Name            string     `gorm:"not null"`
	URL             string     `gorm:"not null"`
	IconKey         string
	IconMode        string `gorm:"not null;default:builtin"`
	IconURL         string `gorm:"not null;default:''"`
	IconAssetID     string `gorm:"not null;default:''"`
	WebCheckEnabled bool   `gorm:"not null"`
	Description     string
	Hidden          bool  `gorm:"not null;default:false"`
	Position        int   `gorm:"not null;check:position >= 0"`
	Revision        int64 `gorm:"not null;default:1"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (ServiceModel) TableName() string { return "services" }

// Matches the session format used by SCS gormstore; not an application DTO.
type SessionModel struct {
	Token  string    `gorm:"column:token;primaryKey;type:varchar(43)"`
	Data   []byte    `gorm:"column:data"`
	Expiry time.Time `gorm:"column:expiry;index"`
}

func (SessionModel) TableName() string { return "sessions" }

type SettingsModel struct {
	ID        int    `gorm:"primaryKey;check:id = 1"`
	Title     string `gorm:"not null"`
	Subtitle  string `gorm:"not null"`
	Theme     string `gorm:"not null"`
	Accent    string `gorm:"not null"`
	Compact   bool   `gorm:"not null"`
	CustomCSS string `gorm:"not null"`
	Revision  int64  `gorm:"not null;default:1"`
}

func (SettingsModel) TableName() string { return "settings" }
