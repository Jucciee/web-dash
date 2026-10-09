package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"homedashboard/internal/domain"
)

var engineReadPath = regexp.MustCompile(`^(/v[0-9]+\.[0-9]+)?/(info|version|containers/json|containers/[a-f0-9]{64}/stats)$`)

func readOnlyDocker(request *http.Request) error {
	if (request.Method == http.MethodGet || request.Method == http.MethodHead) && request.URL.Path == "/_ping" {
		return nil
	}
	if request.Method == http.MethodGet && engineReadPath.MatchString(request.URL.Path) {
		return nil
	}
	return fmt.Errorf("docker operation is not permitted")
}

func (c *Collector) collectDocker(ctx context.Context) domain.DockerSnapshot {
	snapshot := domain.DockerSnapshot{Containers: []domain.ContainerSnapshot{}, Warnings: []string{}, MeasuredAt: time.Now().UTC()}
	opts := []client.Opt{client.WithHostFromEnv(), client.WithTLSClientConfigFromEnv(), client.WithTimeout(8 * time.Second), client.WithHTTPRequestHook(readOnlyDocker)}
	if c.cfg.DockerHost != "" {
		opts = append(opts, client.WithHost(c.cfg.DockerHost))
	}
	engine, err := client.New(opts...)
	if err != nil {
		snapshot.Message = "Docker 연결 설정을 확인해주세요."
		return snapshot
	}
	defer engine.Close()
	rows, err := engine.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		snapshot.Message = "Docker에 연결하지 못했습니다. Engine 실행 상태와 조회 권한을 확인해주세요."
		return snapshot
	}
	snapshot.Available = true
	if info, err := engine.Info(ctx, client.InfoOptions{}); err == nil {
		snapshot.Name, snapshot.OS, snapshot.Version, snapshot.CPUCores = info.Info.Name, info.Info.OperatingSystem, info.Info.ServerVersion, info.Info.NCPU
		if snapshot.OS == "" {
			snapshot.OS = info.Info.OSType
		}
		if info.Info.MemTotal > 0 {
			snapshot.MemoryTotal = uint64(info.Info.MemTotal)
		}
	} else {
		snapshot.Warnings = append(snapshot.Warnings, "Engine 정보 조회 권한이 없어 컨테이너 목록만 표시합니다.")
	}
	sort.Slice(rows.Items, func(i, j int) bool { return containerName(rows.Items[i]) < containerName(rows.Items[j]) })
	snapshot.Containers = make([]domain.ContainerSnapshot, len(rows.Items))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 4)
	for index, row := range rows.Items {
		item := domain.ContainerSnapshot{ID: row.ID, Name: containerName(row), Image: row.Image, Project: row.Labels["com.docker.compose.project"], State: string(row.State), Status: row.Status, Health: "none"}
		if row.Health != nil {
			item.Health = string(row.Health.Status)
		} else {
			for _, health := range []string{"unhealthy", "healthy", "starting"} {
				if strings.Contains(row.Status, "("+health+")") || strings.Contains(row.Status, "health: "+health) {
					item.Health = health
					break
				}
			}
		}
		snapshot.Containers[index] = item
		if item.State != "running" {
			snapshot.Stopped++
			continue
		}
		snapshot.Running++
		wg.Add(1)
		go func(index int, id string) {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				snapshot.Containers[index].StatsError = "수집 시간 초과"
				return
			}
			readCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			stats, err := engine.ContainerStats(readCtx, id, client.ContainerStatsOptions{IncludePreviousSample: true})
			if err != nil {
				snapshot.Containers[index].StatsError = "사용량을 읽지 못했습니다."
				return
			}
			defer stats.Body.Close()
			var sample container.StatsResponse
			if err := json.NewDecoder(io.LimitReader(stats.Body, 1<<20)).Decode(&sample); err != nil {
				snapshot.Containers[index].StatsError = "사용량 응답을 읽지 못했습니다."
				return
			}
			applyStats(&snapshot.Containers[index], sample)
		}(index, row.ID)
	}
	wg.Wait()
	snapshot.MeasuredAt = time.Now().UTC()
	return snapshot
}

func containerName(row container.Summary) string {
	if len(row.Names) > 0 {
		return strings.TrimPrefix(row.Names[0], "/")
	}
	if len(row.ID) > 12 {
		return row.ID[:12]
	}
	return row.ID
}

func applyStats(item *domain.ContainerSnapshot, sample container.StatsResponse) {
	current, previous := sample.CPUStats, sample.PreCPUStats
	cores := current.OnlineCPUs
	if cores == 0 {
		cores = uint32(len(current.CPUUsage.PercpuUsage))
	}
	if !sample.PreRead.IsZero() && current.SystemUsage > previous.SystemUsage && current.CPUUsage.TotalUsage >= previous.CPUUsage.TotalUsage && cores > 0 {
		percent := float64(current.CPUUsage.TotalUsage-previous.CPUUsage.TotalUsage) / float64(current.SystemUsage-previous.SystemUsage) * float64(cores) * 100
		item.CPUPercent = &percent
	}
	memory := sample.MemoryStats
	if memory.Limit > 0 {
		used := memory.Usage
		cached, found := memory.Stats["total_inactive_file"]
		if !found {
			cached = memory.Stats["inactive_file"]
		}
		if cached < used {
			used -= cached
		}
		item.Memory = &domain.ResourceUsage{Used: used, Total: memory.Limit, Percent: float64(used) / float64(memory.Limit) * 100}
	} else if memory.PrivateWorkingSet > 0 {
		item.Memory = &domain.ResourceUsage{Used: memory.PrivateWorkingSet}
	}
	if len(sample.Networks) > 0 {
		var rx, tx uint64
		for _, network := range sample.Networks {
			rx += network.RxBytes
			tx += network.TxBytes
		}
		item.NetworkRX, item.NetworkTX = &rx, &tx
	}
}
