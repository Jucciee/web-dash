package addons

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"homedashboard/internal/domain"
	"homedashboard/internal/services"
)

func clip(text string, limit int) string {
	if utf8.RuneCountInString(text) > limit {
		return string([]rune(text)[:limit]) + "…"
	}
	return text
}
func (s *Service) collect(ctx context.Context, manifest domain.AddonManifest, inputs, secrets map[string]string, preview bool) domain.AddonSnapshot {
	result := domain.AddonSnapshot{MeasuredAt: time.Now().UTC(), Values: []domain.AddonValue{}, Choices: []domain.AddonChoice{}}
	start := time.Now()
	select {
	case s.limit <- struct{}{}:
		defer func() { <-s.limit }()
	case <-ctx.Done():
		result.Message = "연결 테스트 시간이 초과되었습니다."
		return result
	}
	address := render(manifest.Request.URL, inputs)
	if _, err := services.ValidateURL(address); err != nil {
		result.Message = "API 주소와 연결값을 확인해주세요."
		return result
	}
	body := render(manifest.Request.Body, inputs)
	request, err := http.NewRequestWithContext(ctx, manifest.Request.Method, address, strings.NewReader(body))
	if err != nil {
		result.Message = "API 요청 설정을 확인해주세요."
		return result
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "web-dash/addon-v1")
	if manifest.Request.Method == "POST" {
		request.Header.Set("Content-Type", "application/json")
	}
	for _, header := range manifest.Request.Headers {
		request.Header.Set(header.Name, render(header.Value, inputs))
	}
	response, err := s.client.Do(request)
	result.LatencyMS, result.MeasuredAt = time.Since(start).Milliseconds(), time.Now().UTC()
	if err != nil {
		result.Message = "API에 연결하지 못했습니다. 주소·TLS 인증서·서버 실행 상태를 확인해주세요."
		return result
	}
	defer response.Body.Close()
	result.HTTPStatus = response.StatusCode
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		result.Message = fmt.Sprintf("API 응답: HTTP %d", response.StatusCode)
		if response.StatusCode >= 300 && response.StatusCode < 400 {
			result.Message += " · 이동 후의 최종 API 주소를 입력해주세요."
		}
		if response.StatusCode == 401 || response.StatusCode == 403 {
			result.Message += " · 토큰·쿠키와 권한을 확인해주세요."
		}
		return result
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	result.LatencyMS, result.MeasuredAt = time.Since(start).Milliseconds(), time.Now().UTC()
	if err != nil || len(data) > 1<<20 {
		result.Message = "API 응답을 읽지 못했거나 1MiB를 초과했습니다."
		return result
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var root any
	if err := decoder.Decode(&root); err != nil {
		result.Message = "JSON 응답이 아닙니다. JSON을 반환하는 API 주소를 입력해주세요."
		return result
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		result.Message = "API 응답에 여러 JSON 문서가 있습니다."
		return result
	}
	clean := redactor(secrets)
	result.Available = true
	for _, field := range manifest.Fields {
		value, exists := lookup(root, field.Path)
		formatted, supported := formatValue(value, field)
		result.Values = append(result.Values, domain.AddonValue{Label: field.Label, Value: clip(clean(formatted), 512), Unit: field.Unit, Missing: !exists || !supported})
	}
	if preview {
		choices(root, "", 0, clean, &result.Choices)
	}
	return result
}
func choices(value any, path string, depth int, clean func(string) string, out *[]domain.AddonChoice) {
	if len(*out) >= 80 || depth > 12 || clean(path) != path {
		return
	}
	escape := func(key string) string { return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1") }
	switch node := value.(type) {
	case map[string]any:
		*out = append(*out, domain.AddonChoice{Path: path, Value: fmt.Sprintf("%d개 항목", len(node)), Type: "object"})
		keys := make([]string, 0, len(node))
		for key := range node {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if clean(key) != key {
				continue
			}
			choices(node[key], path+"/"+escape(key), depth+1, clean, out)
			if len(*out) >= 80 {
				break
			}
		}
	case []any:
		*out = append(*out, domain.AddonChoice{Path: path, Value: fmt.Sprintf("%d개 항목", len(node)), Type: "array"})
		for index, child := range node {
			choices(child, fmt.Sprintf("%s/%d", path, index), depth+1, clean, out)
			if len(*out) >= 80 {
				break
			}
		}
	case nil:
		*out = append(*out, domain.AddonChoice{Path: path, Value: "null", Type: "null"})
	default:
		kind := "text"
		switch node.(type) {
		case json.Number:
			kind = "number"
		case bool:
			kind = "boolean"
		}
		*out = append(*out, domain.AddonChoice{Path: path, Value: clip(clean(fmt.Sprint(node)), 120), Type: kind})
	}
}
