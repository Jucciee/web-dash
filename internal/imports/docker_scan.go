package imports

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"homedashboard/internal/domain"
	"homedashboard/internal/services"
)

type composeFile struct {
	Path, Directory, Project string
	Services                 map[string]struct{}
}

var composeNames = map[string]bool{"compose.yml": true, "compose.yaml": true, "docker-compose.yml": true, "docker-compose.yaml": true}

func scanCompose(ctx context.Context, folder string) ([]composeFile, string, error) {
	folder = strings.TrimSpace(folder)
	if !filepath.IsAbs(folder) {
		return nil, "", &domain.ValidationError{Message: "서버에서 접근할 수 있는 절대 폴더 경로를 입력해주세요."}
	}
	root, err := filepath.EvalSymlinks(filepath.Clean(folder))
	if err != nil {
		return nil, "", &domain.ImportError{Code: "IMPORT_FOLDER_UNAVAILABLE", Message: "폴더에 접근하지 못했습니다. 서버에 존재하는 경로와 읽기 권한을 확인해주세요."}
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, "", &domain.ValidationError{Message: "설정 파일이 있는 폴더를 입력해주세요."}
	}
	files := []composeFile{}
	visited, totalBytes := 0, int64(0)
	err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return &domain.ImportError{Code: "IMPORT_FOLDER_UNAVAILABLE", Message: "하위 폴더의 읽기 권한을 확인해주세요."}
		}
		visited++
		if visited > 10000 {
			return &domain.ImportError{Code: "IMPORT_LIMIT", Message: "폴더에 항목이 너무 많습니다. Compose 프로젝트가 있는 하위 폴더를 입력해주세요."}
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, name)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return &domain.ValidationError{Message: "폴더 범위를 벗어나는 경로입니다."}
		}
		if entry.IsDir() {
			if name != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" || strings.Count(filepath.ToSlash(rel), "/") >= 4) {
				return filepath.SkipDir
			}
			return nil
		}
		if !composeNames[strings.ToLower(entry.Name())] {
			return nil
		}
		if len(files) >= 50 {
			return &domain.ImportError{Code: "IMPORT_LIMIT", Message: "Compose 파일이 50개를 초과했습니다. 폴더를 나누어 가져와주세요."}
		}
		// Skip non-regular files and links; never follow them out of the chosen root.
		info, err := os.Lstat(name)
		if err != nil || !info.Mode().IsRegular() {
			return &domain.ImportError{Code: "IMPORT_FOLDER_UNAVAILABLE", Message: "Compose 파일을 읽지 못했습니다."}
		}
		file, err := os.Open(name)
		if err != nil {
			return &domain.ImportError{Code: "IMPORT_FOLDER_UNAVAILABLE", Message: "Compose 파일의 읽기 권한을 확인해주세요."}
		}
		data, readErr := io.ReadAll(io.LimitReader(file, (2<<20)+1))
		file.Close()
		if readErr != nil {
			return &domain.ImportError{Code: "IMPORT_PARSE_ERROR", Message: "Compose 파일을 읽지 못했습니다."}
		}
		totalBytes += int64(len(data))
		if len(data) > 2<<20 || totalBytes > 10<<20 {
			return &domain.ImportError{Code: "IMPORT_LIMIT", Message: "파일은 2MiB, 전체 설정은 10MiB까지 분석할 수 있습니다."}
		}
		var config struct {
			Name     string              `yaml:"name"`
			Services map[string]struct{} `yaml:"services"`
		}
		if err := yaml.Unmarshal(data, &config); err != nil {
			return &domain.ImportError{Code: "IMPORT_PARSE_ERROR", Message: fmt.Sprintf("%s의 YAML 문법을 확인해주세요.", rel)}
		}
		files = append(files, composeFile{Path: name, Directory: filepath.Dir(name), Project: config.Name, Services: config.Services})
		return nil
	})
	return files, root, err
}

func validatePublicHost(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "://") || strings.ContainsAny(raw, "/?#@ \t\r\n\\") || raw == "" {
		return "", &domain.ValidationError{Message: "접속할 호스트 이름 또는 IP만 입력해주세요. 예: localhost, 192.168.1.10"}
	}
	host := strings.Trim(raw, "[]")
	if strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return "", &domain.ValidationError{Message: "포트를 제외한 호스트 이름 또는 IP를 입력해주세요."}
	}
	u, err := url.Parse("http://" + net.JoinHostPort(host, "8080"))
	if err != nil || u.Hostname() == "" {
		return "", &domain.ValidationError{Message: "올바른 호스트 이름 또는 IP를 입력해주세요."}
	}
	return host, nil
}

var windowsMount = regexp.MustCompile(`^/(?:run/desktop/mnt/host|host_mnt|mnt)/([a-zA-Z])/(.*)$`)

func dockerPath(raw string) string {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	if match := windowsMount.FindStringSubmatch(raw); match != nil {
		raw = match[1] + ":/" + match[2]
	}
	raw = strings.TrimRight(raw, "/")
	if len(raw) > 1 && raw[1] == ':' {
		raw = strings.ToLower(raw)
	}
	return raw
}

func matchesCompose(file composeFile, item domain.RunningContainer) bool {
	if _, ok := file.Services[item.Service]; !ok || item.Project == "" {
		return false
	}
	if item.ConfigFiles != "" {
		for _, name := range strings.Split(item.ConfigFiles, ",") {
			if dockerPath(name) == dockerPath(file.Path) {
				return true
			}
		}
		// Explicit config-file labels take precedence over a similarly named project.
		return false
	}
	if item.WorkingDir != "" {
		return dockerPath(item.WorkingDir) == dockerPath(file.Directory)
	}
	// Without path labels, a project name alone cannot prove folder membership.
	return false
}

func dockerCandidates(files []composeFile, containers []domain.RunningContainer, publicHost string) ([]domain.ImportCandidate, []string) {
	candidates := []domain.ImportCandidate{}
	warnings := []string{}
	matched := map[string]bool{}
	for _, file := range files {
		for _, item := range containers {
			if !matchesCompose(file, item) {
				continue
			}
			matched[file.Path+"\x00"+item.Service] = true
			if item.PreferredURL != "" {
				if _, err := services.ValidateService(domain.ServiceInput{Name: item.Service, URL: item.PreferredURL}); err == nil {
					candidates = append(candidates, domain.ImportCandidate{Name: item.Service, URL: item.PreferredURL, File: file.Path, Upstreams: []string{item.Name}, Warnings: []string{"실행 중 · homepage.href 라벨의 주소"}})
					continue
				}
			}
			ports := append([]domain.PublishedPort(nil), item.Ports...)
			sort.Slice(ports, func(i, j int) bool { return ports[i].Public < ports[j].Public })
			found := false
			for _, port := range ports {
				if port.Protocol != "tcp" || port.Public == 0 {
					continue
				}
				host := publicHost
				if port.IP != "" && port.IP != "0.0.0.0" && port.IP != "::" {
					host = port.IP
				}
				scheme := "http"
				if port.Private == 443 || port.Private == 8443 {
					scheme = "https"
				}
				address := scheme + "://" + net.JoinHostPort(host, strconv.Itoa(int(port.Public)))
				name := item.Service
				if len(ports) > 1 {
					name += " · " + strconv.Itoa(int(port.Public))
				}
				candidates = append(candidates, domain.ImportCandidate{Name: name, URL: address, File: file.Path, Upstreams: []string{fmt.Sprintf("%s · %d → %d/tcp", item.Name, port.Public, port.Private)}, Warnings: []string{"실행 중인 컨테이너의 공개 포트입니다. HTTP/HTTPS 서비스인지 접속 주소를 확인해주세요."}})
				found = true
			}
			if !found {
				warnings = append(warnings, fmt.Sprintf("%s: 실행 중이지만 공개 TCP 포트가 없습니다. 리버스 프록시 주소는 Nginx/Caddy에서 가져오세요.", item.Service))
			}
		}
		keys := make([]string, 0, len(file.Services))
		for service := range file.Services {
			keys = append(keys, service)
		}
		sort.Strings(keys)
		for _, service := range keys {
			if !matched[file.Path+"\x00"+service] {
				warnings = append(warnings, fmt.Sprintf("%s: 이 Compose 파일과 연결된 실행 중 컨테이너를 찾지 못했습니다.", service))
			}
		}
	}
	return candidates, warnings
}
