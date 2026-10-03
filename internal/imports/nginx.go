package imports

import (
	"fmt"
	"io"
	"net"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	crossplane "github.com/nginxinc/nginx-go-crossplane"
	"homedashboard/internal/domain"
)

func virtualFiles(input []domain.ImportFile, entry string) (map[string]string, string, error) {
	files := map[string]string{}
	total := 0
	for _, file := range input {
		name := strings.ReplaceAll(file.Path, "\\", "/")
		clean := path.Clean(name)
		if clean != name || strings.HasPrefix(name, "/") || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.ContainsAny(name, ":\x00") {
			return nil, "", &domain.ValidationError{Message: "설정 파일은 중복되지 않는 상대 경로로 올려주세요."}
		}
		if _, exists := files[clean]; exists {
			return nil, "", &domain.ValidationError{Message: "파일 경로가 중복되었습니다."}
		}
		total += len(file.Content)
		if len(file.Content) > 2<<20 || total > 10<<20 {
			return nil, "", &domain.ImportError{Code: "IMPORT_LIMIT", Message: "파일은 2MiB, 전체 설정은 10MiB까지 분석할 수 있습니다."}
		}
		files[clean] = strings.TrimPrefix(file.Content, "\uFEFF")
	}
	entry = strings.ReplaceAll(entry, "\\", "/")
	if _, exists := files[entry]; !exists {
		return nil, "", &domain.ValidationError{Message: "분석을 시작할 파일을 선택해주세요."}
	}
	return files, entry, nil
}

func parseNginx(files map[string]string, entry string) ([]domain.ImportCandidate, []string, error) {
	options := &crossplane.ParseOptions{CombineConfigs: true, SkipDirectiveContextCheck: true, StopParsingOnError: true,
		Open: func(name string) (io.ReadCloser, error) {
			name = filepath.ToSlash(name)
			if content, ok := files[name]; ok {
				return io.NopCloser(strings.NewReader(content)), nil
			}
			return nil, &domain.ImportError{Code: "IMPORT_MISSING_FILE", Message: fmt.Sprintf("참조 파일 %s을 함께 올려주세요. 절대 include 경로는 올린 파일의 상대 경로로 바꿔주세요.", name)}
		},
		Glob: func(pattern string) ([]string, error) {
			pattern = filepath.ToSlash(pattern)
			matches := []string{}
			for name := range files {
				ok, err := path.Match(pattern, name)
				if err != nil {
					return nil, err
				}
				if ok {
					matches = append(matches, name)
				}
			}
			if len(matches) == 0 {
				return nil, &domain.ImportError{Code: "IMPORT_MISSING_FILE", Message: "Nginx include 파일을 함께 올려주세요."}
			}
			sort.Strings(matches)
			return matches, nil
		},
	}
	payload, err := crossplane.Parse(entry, options)
	if err != nil {
		if failure, ok := err.(*domain.ImportError); ok {
			return nil, nil, failure
		}
		return nil, nil, &domain.ImportError{Code: "IMPORT_PARSE_ERROR", Message: "Nginx 설정의 문법과 include 파일을 확인해주세요."}
	}
	if payload.Status != "ok" {
		return nil, nil, &domain.ImportError{Code: "IMPORT_PARSE_ERROR", Message: "Nginx 설정의 괄호, 세미콜론, include 파일을 확인해주세요."}
	}
	candidates := []domain.ImportCandidate{}
	warnings := []string{}
	var walk func(crossplane.Directives, string, int)
	walk = func(nodes crossplane.Directives, file string, depth int) {
		if depth > 64 {
			warnings = append(warnings, "너무 깊은 설정 블록을 건너뛰었습니다.")
			return
		}
		for _, node := range nodes {
			location := file
			if node.File != "" {
				location = node.File
			}
			if node.Directive == "server" && len(node.Block) > 0 {
				candidates = append(candidates, nginxServer(node, location)...)
			} else {
				walk(node.Block, location, depth+1)
			}
		}
	}
	for _, config := range payload.Config {
		walk(config.Parsed, config.File, 0)
	}
	return candidates, warnings, nil
}

func nginxServer(server *crossplane.Directive, file string) []domain.ImportCandidate {
	names := []string{}
	port, scheme := "80", "http"
	for _, node := range server.Block {
		if node.Directive == "server_name" {
			names = append(names, node.Args...)
		}
		if node.Directive == "listen" && len(node.Args) > 0 {
			secure := false
			for _, arg := range node.Args {
				if arg == "ssl" {
					secure = true
				}
			}
			if scheme == "https" && !secure {
				continue
			}
			if secure {
				scheme = "https"
			}
			value := node.Args[0]
			if _, listenPort, err := net.SplitHostPort(value); err == nil {
				value = listenPort
			} else if index := strings.LastIndex(value, ":"); index >= 0 {
				value = value[index+1:]
			}
			if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 && parsed <= 65535 {
				port = value
			}
		}
	}
	type route struct {
		prefix    string
		upstreams []string
		warnings  []string
		line      int
	}
	routes := []route{}
	var collect func(crossplane.Directives, string, []string, int)
	collect = func(nodes crossplane.Directives, prefix string, warnings []string, depth int) {
		if depth > 64 {
			return
		}
		for _, node := range nodes {
			if node.Directive == "proxy_pass" && len(node.Args) > 0 {
				routes = append(routes, route{prefix: prefix, upstreams: []string{node.Args[0]}, warnings: append([]string{}, warnings...), line: node.Line})
			}
			nextPrefix, nextWarnings := prefix, warnings
			if node.Directive == "location" && len(node.Args) > 0 {
				value := node.Args[0]
				if (value == "=" || value == "^~") && len(node.Args) > 1 {
					value = node.Args[1]
				}
				if strings.HasPrefix(value, "/") && !strings.ContainsAny(value, "*$?#\\") {
					nextPrefix = value
				} else {
					nextPrefix = ""
					nextWarnings = []string{"정규식·변수·이름이 있는 location입니다. 실제 접속 경로를 입력해주세요."}
				}
			}
			collect(node.Block, nextPrefix, nextWarnings, depth+1)
		}
	}
	collect(server.Block, "/", nil, 0)
	if len(routes) == 0 {
		routes = append(routes, route{prefix: "/", line: server.Line})
	}
	if len(names) == 0 {
		names = []string{"_"}
	}
	result := []domain.ImportCandidate{}
	for _, host := range names {
		for _, item := range routes {
			candidate := domain.ImportCandidate{Name: serviceName(host), File: file, Line: item.line, Upstreams: item.upstreams, Warnings: append([]string{}, item.warnings...)}
			if item.prefix != "/" && item.prefix != "" {
				candidate.Name += " · " + strings.Trim(item.prefix, "/")
			}
			if host == "_" || strings.ContainsAny(host, "*~$\\") {
				candidate.Name = "Nginx 서비스"
				candidate.Warnings = append(candidate.Warnings, "기본 서버·와일드카드·변수 주소입니다. 실제 접속 주소를 입력해주세요.")
			} else {
				address := host
				if (scheme == "http" && port != "80") || (scheme == "https" && port != "443") {
					address = net.JoinHostPort(host, port)
				}
				if item.prefix != "" {
					candidate.URL = scheme + "://" + address
					if item.prefix != "/" {
						candidate.URL += item.prefix
					}
				}
			}
			result = append(result, candidate)
		}
	}
	return result
}

func serviceName(host string) string {
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	if net.ParseIP(strings.Trim(host, "[]")) == nil {
		host = strings.Split(host, ".")[0]
	}
	if host == "" {
		return "서비스"
	}
	return host
}
