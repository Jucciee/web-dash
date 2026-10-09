package telemetry

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"homedashboard/internal/domain"
)

func (c *Collector) collectHost(ctx context.Context) domain.HostSnapshot {
	env := common.EnvMap{}
	if c.cfg.HostProc != "" {
		env[common.HostProcEnvKey] = c.cfg.HostProc
	}
	if c.cfg.HostSys != "" {
		env[common.HostSysEnvKey] = c.cfg.HostSys
	}
	if c.cfg.HostRoot != "" {
		env[common.HostRootEnvKey] = c.cfg.HostRoot
	}
	ctx = context.WithValue(ctx, common.EnvKey, env)
	snapshot := domain.HostSnapshot{OS: runtime.GOOS, Scope: "system", CPUCores: runtime.NumCPU(), Warnings: []string{}}
	snapshot.Name, _ = os.Hostname()
	if _, err := os.Stat("/.dockerenv"); err == nil || os.Getenv("container") != "" {
		snapshot.Scope = "runtime"
	}
	if c.cfg.HostProc != "" || c.cfg.HostSys != "" || c.cfg.HostRoot != "" {
		if runtime.GOOS == "linux" && c.cfg.HostProc != "" && c.cfg.HostSys != "" && c.cfg.HostRoot != "" {
			snapshot.Scope = "host"
		} else {
			snapshot.Warnings = append(snapshot.Warnings, "실제 호스트 표시에는 Linux의 proc, sys, root 마운트가 모두 필요합니다.")
		}
	}
	if c.cfg.HostName != "" {
		snapshot.Name = c.cfg.HostName
	}
	if cores, err := cpu.CountsWithContext(ctx, true); err == nil && cores > 0 {
		snapshot.CPUCores = cores
	}
	if values, err := cpu.PercentWithContext(ctx, 200*time.Millisecond, false); err == nil && len(values) > 0 {
		snapshot.CPUPercent = &values[0]
	} else {
		snapshot.Warnings = append(snapshot.Warnings, "CPU 사용량을 읽지 못했습니다.")
	}
	if memory, err := mem.VirtualMemoryWithContext(ctx); err == nil && memory.Total > 0 {
		snapshot.Memory = &domain.ResourceUsage{Used: memory.Used, Total: memory.Total, Percent: memory.UsedPercent}
	} else {
		snapshot.Warnings = append(snapshot.Warnings, "메모리 사용량을 읽지 못했습니다.")
	}
	diskPath := c.cfg.HostDisk
	if diskPath == "" {
		diskPath = "/"
		if runtime.GOOS == "windows" {
			cwd, _ := os.Getwd()
			diskPath = filepath.VolumeName(cwd) + string(os.PathSeparator)
		} else if c.cfg.HostRoot != "" {
			diskPath = c.cfg.HostRoot
		}
	}
	if usage, err := disk.UsageWithContext(ctx, diskPath); err == nil && usage.Total > 0 {
		snapshot.Disk = &domain.DiskUsage{Path: diskPath, ResourceUsage: domain.ResourceUsage{Used: usage.Used, Total: usage.Total, Percent: usage.UsedPercent}}
	} else {
		snapshot.Warnings = append(snapshot.Warnings, "선택한 디스크 경로의 사용량을 읽지 못했습니다.")
	}
	if uptime, err := host.UptimeWithContext(ctx); err == nil {
		snapshot.UptimeSeconds = &uptime
	}
	snapshot.Available = snapshot.CPUPercent != nil || snapshot.Memory != nil || snapshot.Disk != nil
	snapshot.MeasuredAt = time.Now().UTC()
	return snapshot
}
