package imports

import (
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"homedashboard/internal/domain"
)

// Use Caddy's official lexer on inert uploaded bytes. Do not call adapt/Parse:
// those can resolve environment values, modules and files on the API server.
type caddyNode struct {
	words    []caddyfile.Token
	block    []caddyNode
	hasBlock bool
}

func caddyNodes(tokens []caddyfile.Token, cursor *int, nested bool, depth int) ([]caddyNode, error) {
	if depth > 64 {
		return nil, fmt.Errorf("nesting limit")
	}
	nodes := []caddyNode{}
	for *cursor < len(tokens) {
		if tokens[*cursor].Text == "}" {
			if !nested {
				return nil, fmt.Errorf("unexpected closing block")
			}
			*cursor++
			return nodes, nil
		}
		node := caddyNode{}
		line := tokens[*cursor].Line
		for *cursor < len(tokens) {
			token := tokens[*cursor]
			if token.Text == "{" {
				*cursor++
				block, err := caddyNodes(tokens, cursor, true, depth+1)
				if err != nil {
					return nil, err
				}
				node.block, node.hasBlock = block, true
				break
			}
			if token.Text == "}" || (len(node.words) > 0 && token.Line != line && !strings.HasSuffix(node.words[len(node.words)-1].Text, ",")) {
				break
			}
			node.words = append(node.words, token)
			line = token.Line
			*cursor++
		}
		if len(node.words) == 0 && !node.hasBlock {
			return nil, fmt.Errorf("empty directive")
		}
		nodes = append(nodes, node)
	}
	if nested {
		return nil, fmt.Errorf("unclosed block")
	}
	return nodes, nil
}

func parseCaddy(files map[string]string, entry string) ([]domain.ImportCandidate, []string, error) {
	expandedTokens := 0
	var load func(string, map[string]bool) ([]caddyNode, error)
	load = func(name string, active map[string]bool) ([]caddyNode, error) {
		if active[name] || len(active) > 32 {
			return nil, &domain.ImportError{Code: "IMPORT_PARSE_ERROR", Message: "Caddy import가 순환하거나 너무 깊습니다."}
		}
		content, ok := files[name]
		if !ok {
			return nil, &domain.ImportError{Code: "IMPORT_MISSING_FILE", Message: "참조한 Caddy 파일을 함께 올려주세요."}
		}
		tokens, err := caddyfile.Tokenize([]byte(content), name)
		if err != nil {
			return nil, &domain.ImportError{Code: "IMPORT_PARSE_ERROR", Message: "Caddyfile의 따옴표와 블록 문법을 확인해주세요."}
		}
		expandedTokens += len(tokens)
		if expandedTokens > 100000 {
			return nil, &domain.ImportError{Code: "IMPORT_LIMIT", Message: "Caddy 설정과 import 확장이 너무 큽니다. 파일을 나누어 가져와주세요."}
		}
		cursor := 0
		nodes, err := caddyNodes(tokens, &cursor, false, 0)
		if err != nil {
			return nil, &domain.ImportError{Code: "IMPORT_PARSE_ERROR", Message: "Caddyfile의 여는 괄호와 닫는 괄호를 확인해주세요."}
		}
		active[name] = true
		defer delete(active, name)
		result := []caddyNode{}
		for _, node := range nodes {
			if len(node.words) > 0 && node.words[0].Text == "import" {
				if len(node.words) != 2 {
					return nil, &domain.ImportError{Code: "IMPORT_UNSUPPORTED", Message: "인자가 있는 Caddy import는 내용을 펼친 뒤 올려주세요."}
				}
				pattern := node.words[1].Text
				if strings.HasPrefix(pattern, "/") || strings.Contains(pattern, "..") {
					return nil, &domain.ImportError{Code: "IMPORT_MISSING_FILE", Message: "Caddy import는 올린 파일의 상대 경로로 바꿔주세요."}
				}
				pattern = path.Join(path.Dir(name), pattern)
				matches := []string{}
				for file := range files {
					if match, _ := path.Match(pattern, file); match {
						matches = append(matches, file)
					}
				}
				sort.Strings(matches)
				if len(matches) == 0 {
					return nil, &domain.ImportError{Code: "IMPORT_MISSING_FILE", Message: fmt.Sprintf("Caddy import %s에 해당하는 파일을 함께 올려주세요.", pattern)}
				}
				for _, file := range matches {
					imported, err := load(file, active)
					if err != nil {
						return nil, err
					}
					result = append(result, imported...)
				}
			} else {
				result = append(result, node)
			}
		}
		return result, nil
	}
	nodes, err := load(entry, map[string]bool{})
	if err != nil {
		return nil, nil, err
	}
	// The brace-free single-site form starts with its address on the first line.
	if len(nodes) > 1 && len(nodes[0].words) > 0 && !nodes[0].hasBlock {
		nodes = []caddyNode{{words: nodes[0].words, block: nodes[1:], hasBlock: true}}
	}
	snippets := map[string][]caddyNode{}
	for _, node := range nodes {
		if len(node.words) == 1 && strings.HasPrefix(node.words[0].Text, "(") && strings.HasSuffix(node.words[0].Text, ")") {
			snippets[strings.Trim(node.words[0].Text, "()")] = node.block
		}
	}
	candidates := []domain.ImportCandidate{}
	warnings := []string{}
	for _, site := range nodes {
		if len(site.words) == 0 || strings.HasPrefix(site.words[0].Text, "(") || strings.HasPrefix(site.words[0].Text, "&(") {
			continue
		}
		upstreams := []string{}
		localWarnings := []string{}
		var walk func([]caddyNode, int, map[string]bool)
		walk = func(body []caddyNode, depth int, active map[string]bool) {
			if depth > 64 {
				localWarnings = append(localWarnings, "설정 블록이 너무 깊습니다.")
				return
			}
			for _, node := range body {
				if len(node.words) == 0 {
					continue
				}
				if node.words[0].Text == "import" {
					if len(node.words) == 2 {
						name := node.words[1].Text
						if snippet, ok := snippets[name]; ok && !active[name] {
							active[name] = true
							walk(snippet, depth+1, active)
							delete(active, name)
							continue
						}
					}
					localWarnings = append(localWarnings, "사이트 내부 import는 업스트림을 확인하지 못했습니다. 접속 주소는 직접 확인해주세요.")
				}
				if node.words[0].Text == "reverse_proxy" || node.words[0].Text == "to" {
					for _, word := range node.words[1:] {
						if !strings.HasPrefix(word.Text, "@") && !strings.HasPrefix(word.Text, "/") {
							upstreams = append(upstreams, word.Text)
						}
					}
				}
				walk(node.block, depth+1, active)
			}
		}
		walk(site.block, 0, map[string]bool{})
		for _, word := range site.words {
			for _, raw := range strings.Split(word.Text, ",") {
				raw = strings.TrimSpace(raw)
				if raw == "" {
					continue
				}
				candidate := domain.ImportCandidate{Name: serviceName(raw), File: word.File, Line: word.Line, Upstreams: upstreams, Warnings: append([]string{}, localWarnings...)}
				if strings.ContainsAny(raw, "*{}") || strings.HasPrefix(raw, ":") {
					candidate.Name = "Caddy 서비스"
					candidate.Warnings = append(candidate.Warnings, "와일드카드·환경변수·포트 전용 주소입니다. 실제 접속 주소를 입력해주세요.")
				} else {
					address := raw
					if !strings.Contains(address, "://") {
						scheme := "https"
						if strings.HasSuffix(address, ":80") {
							scheme = "http"
						}
						address = scheme + "://" + address
					}
					parsed, err := url.Parse(address)
					if err == nil && parsed.Hostname() != "" && parsed.User == nil {
						candidate.URL = address
					} else {
						candidate.Warnings = append(candidate.Warnings, "접속 주소를 입력해주세요.")
					}
				}
				candidates = append(candidates, candidate)
			}
		}
	}
	return candidates, warnings, nil
}
