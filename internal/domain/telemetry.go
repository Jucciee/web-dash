package domain

import "time"

type ResourceUsage struct {
	Used    uint64  `json:"used"`
	Total   uint64  `json:"total"`
	Percent float64 `json:"percent"`
}

type DiskUsage struct {
	Path string `json:"path"`
	ResourceUsage
}

type HostSnapshot struct {
	Available     bool           `json:"available"`
	Name          string         `json:"name"`
	OS            string         `json:"os"`
	Scope         string         `json:"scope"`
	CPUPercent    *float64       `json:"cpuPercent"`
	CPUCores      int            `json:"cpuCores"`
	Memory        *ResourceUsage `json:"memory"`
	Disk          *DiskUsage     `json:"disk"`
	UptimeSeconds *uint64        `json:"uptimeSeconds"`
	MeasuredAt    time.Time      `json:"measuredAt"`
	Warnings      []string       `json:"warnings"`
}

type ContainerSnapshot struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Image      string         `json:"image"`
	Project    string         `json:"project"`
	State      string         `json:"state"`
	Status     string         `json:"status"`
	Health     string         `json:"health"`
	CPUPercent *float64       `json:"cpuPercent"`
	Memory     *ResourceUsage `json:"memory"`
	NetworkRX  *uint64        `json:"networkRx"`
	NetworkTX  *uint64        `json:"networkTx"`
	StatsError string         `json:"statsError"`
}

type DockerSnapshot struct {
	Available   bool                `json:"available"`
	Name        string              `json:"name"`
	OS          string              `json:"os"`
	Version     string              `json:"version"`
	CPUCores    int                 `json:"cpuCores"`
	MemoryTotal uint64              `json:"memoryTotal"`
	Running     int                 `json:"running"`
	Stopped     int                 `json:"stopped"`
	Containers  []ContainerSnapshot `json:"containers"`
	MeasuredAt  time.Time           `json:"measuredAt"`
	Message     string              `json:"message"`
	Warnings    []string            `json:"warnings"`
}
