package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("revision conflict")
	ErrGroupNotEmpty = errors.New("group not empty")
	ErrAlreadySetup  = errors.New("setup already completed")
	ErrUnauthorized  = errors.New("invalid credentials")
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type Service struct {
	ID              string    `json:"id"`
	GroupID         string    `json:"groupId"`
	Name            string    `json:"name"`
	URL             string    `json:"url"`
	IconKey         string    `json:"iconKey"`
	IconMode        string    `json:"iconMode"`
	IconURL         string    `json:"iconUrl"`
	IconAssetID     string    `json:"iconAssetId"`
	WebCheckEnabled bool      `json:"webCheckEnabled"`
	Description     string    `json:"description"`
	Hidden          bool      `json:"hidden"`
	Position        int       `json:"position"`
	Revision        int64     `json:"revision"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Group struct {
	ID          string    `json:"id"`
	DashboardID string    `json:"dashboardId"`
	Name        string    `json:"name"`
	Position    int       `json:"position"`
	Revision    int64     `json:"revision"`
	Services    []Service `json:"services"`
}

type Dashboard struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Revision int64   `json:"revision"`
	Groups   []Group `json:"groups"`
}

type ServiceInput struct {
	GroupID         string `json:"groupId" binding:"required,uuid"`
	Name            string `json:"name" binding:"required,max=80"`
	URL             string `json:"url" binding:"required,max=2048"`
	IconKey         string `json:"iconKey" binding:"max=32"`
	IconMode        string `json:"iconMode" binding:"omitempty,oneof=builtin site url upload"`
	IconURL         string `json:"iconUrl" binding:"max=2048"`
	IconAssetID     string `json:"iconAssetId" binding:"max=64"`
	WebCheckEnabled *bool  `json:"webCheckEnabled"`
	Description     string `json:"description" binding:"max=500"`
	Hidden          bool   `json:"hidden"`
}

type ServicePatch struct {
	GroupID         *string `json:"groupId" binding:"omitempty,uuid"`
	Name            *string `json:"name" binding:"omitempty,min=1,max=80"`
	URL             *string `json:"url" binding:"omitempty,min=1,max=2048"`
	IconKey         *string `json:"iconKey" binding:"omitempty,max=32"`
	IconMode        *string `json:"iconMode" binding:"omitempty,oneof=builtin site url upload"`
	IconURL         *string `json:"iconUrl" binding:"omitempty,max=2048"`
	IconAssetID     *string `json:"iconAssetId" binding:"omitempty,max=64"`
	WebCheckEnabled *bool   `json:"webCheckEnabled"`
	Description     *string `json:"description" binding:"omitempty,max=500"`
	Hidden          *bool   `json:"hidden"`
}

type LayoutGroup struct {
	GroupID    string   `json:"groupId" binding:"required,uuid"`
	ServiceIDs []string `json:"serviceIds" binding:"required,dive,uuid"`
}

type Layout struct {
	Groups []LayoutGroup `json:"groups" binding:"required,dive"`
}
