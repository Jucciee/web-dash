package domain

import "time"

type ImportError struct{ Code, Message string }

func (e *ImportError) Error() string { return e.Message }

type ImportFile struct {
	Path    string `json:"path" binding:"required,max=240"`
	Content string `json:"content" binding:"required"`
}

type ImportInput struct {
	DashboardID string       `json:"dashboardId" binding:"required,uuid"`
	Kind        string       `json:"kind" binding:"required,oneof=nginx caddy"`
	SourceName  string       `json:"sourceName" binding:"max=80"`
	EntryPath   string       `json:"entryPath" binding:"required,max=240"`
	Files       []ImportFile `json:"files" binding:"required,min=1,max=50,dive"`
}

type DockerScanInput struct {
	DashboardID string `json:"dashboardId" binding:"required,uuid"`
	Folder      string `json:"folder" binding:"required,max=2048"`
	PublicHost  string `json:"publicHost" binding:"required,max=253"`
}

type ImportCandidate struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	URL               string   `json:"url"`
	File              string   `json:"file"`
	Line              int      `json:"line"`
	Upstreams         []string `json:"upstreams"`
	Warnings          []string `json:"warnings"`
	Diff              string   `json:"diff"`
	ExistingServiceID string   `json:"existingServiceId"`
}

type ImportPreview struct {
	ID                string            `json:"id"`
	DashboardID       string            `json:"dashboardId"`
	Kind              string            `json:"kind"`
	SourceName        string            `json:"sourceName"`
	DashboardRevision int64             `json:"dashboardRevision"`
	ExpiresAt         time.Time         `json:"expiresAt"`
	Candidates        []ImportCandidate `json:"candidates"`
	Warnings          []string          `json:"warnings"`
}

type ImportSelection struct {
	CandidateID string `json:"candidateId" binding:"required,uuid"`
	ServiceInput
}

type ImportCommit struct {
	DashboardRevision int64             `json:"dashboardRevision" binding:"required,min=1"`
	Items             []ImportSelection `json:"items" binding:"required,min=1,max=500,dive"`
}

type ImportResult struct {
	ServiceIDs        []string `json:"serviceIds"`
	DashboardRevision int64    `json:"dashboardRevision"`
}

type RunningContainer struct {
	Name, Project, Service, WorkingDir, ConfigFiles, PreferredURL string
	Ports                                                         []PublishedPort
}

type PublishedPort struct {
	IP              string
	Public, Private uint16
	Protocol        string
}
