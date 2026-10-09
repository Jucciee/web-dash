package addons

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"homedashboard/internal/domain"
)

var variable = regexp.MustCompile(`\{\{([A-Za-z][A-Za-z0-9_]{0,39})(\|(url|json|base64))?\}\}`)
var inputKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,39}$`)
var headerName = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")

func invalid(message string) error { return &domain.ValidationError{Message: message} }

func ValidateManifest(m domain.AddonManifest) (domain.AddonManifest, error) {
	if m.SchemaVersion != 1 {
		return m, invalid("web-dash 애드온 schemaVersion 1 설정 파일을 선택해주세요.")
	}
	m.Name = strings.TrimSpace(m.Name)
	if m.Name == "" || utf8.RuneCountInString(m.Name) > 80 {
		return m, invalid("애드온 이름을 1~80자로 입력해주세요.")
	}
	if m.Icon == "" {
		m.Icon = "server"
	}
	switch m.Icon {
	case "server", "photos", "media", "files", "code", "home", "network", "database":
	default:
		return m, invalid("아이콘을 목록에서 선택해주세요.")
	}
	if m.Display == "" {
		m.Display = "stats"
	}
	if m.Display != "stats" && m.Display != "list" {
		return m, invalid("표시 방식은 stats 또는 list로 설정해주세요.")
	}
	switch m.RefreshSeconds {
	case 0, 10, 30, 60, 300:
	default:
		return m, invalid("갱신 간격은 수동 또는 10·30·60·300초로 설정해주세요.")
	}
	if m.Request.Method == "" {
		m.Request.Method = "GET"
	}
	if m.Request.Method != "GET" && m.Request.Method != "POST" {
		return m, invalid("요청 방식은 GET 또는 POST로 설정해주세요.")
	}
	if m.Request.URL == "" || len(m.Request.URL) > 2048 || len(m.Request.Body) > 16<<10 || (m.Request.Method == "GET" && m.Request.Body != "") {
		return m, invalid("API 주소와 요청 본문을 확인해주세요. GET은 본문을 사용하지 않습니다.")
	}
	if len(m.Inputs) > 16 || len(m.Request.Headers) > 16 || len(m.Fields) < 1 || len(m.Fields) > 16 {
		return m, invalid("입력·헤더·표시할 값은 각각 최대 16개이며 표시할 값은 하나 이상 필요합니다.")
	}
	if m.Inputs == nil {
		m.Inputs = []domain.AddonInput{}
	}
	if m.Request.Headers == nil {
		m.Request.Headers = []domain.AddonHeader{}
	}
	keys := map[string]domain.AddonInput{}
	for i, input := range m.Inputs {
		if !inputKey.MatchString(input.Key) || input.Key == "constructor" || input.Key == "prototype" {
			return m, invalid("입력 키는 영문자로 시작하는 영문·숫자·밑줄 40자 이내로 설정해주세요.")
		}
		if _, exists := keys[input.Key]; exists {
			return m, invalid("입력 키가 중복되었습니다.")
		}
		if input.Type == "" {
			input.Type = "text"
		}
		if input.Label == "" {
			input.Label = input.Key
		}
		if (input.Type != "text" && input.Type != "secret") || utf8.RuneCountInString(input.Label) > 80 || len(input.Default) > 4096 {
			return m, invalid("입력 항목의 이름·종류·기본값을 확인해주세요.")
		}
		if input.Type == "secret" && input.Default != "" {
			return m, invalid("토큰·쿠키 등 보안 입력에는 기본값을 넣지 마세요. 연결 값에서 입력해주세요.")
		}
		m.Inputs[i], keys[input.Key] = input, input
	}
	checkTemplate := func(template string) error {
		for _, match := range variable.FindAllStringSubmatch(template, -1) {
			if _, ok := keys[match[1]]; !ok {
				return invalid("템플릿의 입력 키가 정의되지 않았습니다: " + match[1])
			}
		}
		left := variable.ReplaceAllString(template, "")
		if strings.Contains(left, "{{") || strings.Contains(left, "}}") {
			return invalid("입력값은 {{key}}, {{key|url}}, {{key|json}}, {{key|base64}}로 참조해주세요.")
		}
		return nil
	}
	for _, template := range []string{m.Request.URL, m.Request.Body} {
		if err := checkTemplate(template); err != nil {
			return m, err
		}
	}
	headers := map[string]bool{}
	for _, header := range m.Request.Headers {
		name := strings.ToLower(header.Name)
		if !headerName.MatchString(header.Name) || headers[name] || len(header.Name) > 80 || len(header.Value) > 8192 || strings.ContainsAny(header.Value, "\r\n\x00") {
			return m, invalid("헤더 이름·값·중복을 확인해주세요.")
		}
		switch name {
		case "host", "connection", "content-length", "transfer-encoding", "proxy-authorization", "proxy-connection", "upgrade", "trailer", "te":
			return m, invalid("이 헤더는 직접 설정할 수 없습니다: " + header.Name)
		}
		if err := checkTemplate(header.Value); err != nil {
			return m, err
		}
		switch name {
		case "authorization", "cookie", "x-api-key", "api-key", "x-auth-token":
			secret := false
			for _, match := range variable.FindAllStringSubmatch(header.Value, -1) {
				secret = secret || keys[match[1]].Type == "secret"
			}
			if !secret {
				return m, invalid("인증 헤더에는 secret 입력을 참조해주세요. 예: Bearer {{token}}")
			}
		}
		headers[name] = true
	}
	for i, field := range m.Fields {
		field.Label = strings.TrimSpace(field.Label)
		if field.Format == "" {
			field.Format = "text"
		}
		if field.Scale == 0 {
			field.Scale = 1
		}
		if field.ValueMap == nil {
			field.ValueMap = map[string]string{}
		}
		if field.Label == "" || utf8.RuneCountInString(field.Label) > 80 || len(field.Path) > 512 || utf8.RuneCountInString(field.Unit) > 24 || field.Decimals < 0 || field.Decimals > 4 || math.IsNaN(field.Scale) || math.IsInf(field.Scale, 0) || math.Abs(field.Scale) > 1e9 || len(field.ValueMap) > 32 {
			return m, invalid("표시할 값의 이름·경로·단위·계산 설정을 확인해주세요.")
		}
		if _, err := pointerParts(field.Path); err != nil {
			return m, err
		}
		switch field.Format {
		case "text", "number", "percent", "bytes", "duration", "boolean", "count":
		default:
			return m, invalid("표시 형식을 목록에서 선택해주세요.")
		}
		for k, v := range field.ValueMap {
			if len(k) > 120 || utf8.RuneCountInString(v) > 120 {
				return m, invalid("값 이름 변환은 120자까지 입력할 수 있습니다.")
			}
		}
		m.Fields[i] = field
	}
	encoded, _ := json.Marshal(m)
	if len(encoded) > 48<<10 {
		return m, invalid("애드온 설정은 48KiB까지 저장할 수 있습니다.")
	}
	return m, nil
}

func render(template string, values map[string]string) string {
	return variable.ReplaceAllStringFunc(template, func(token string) string {
		match := variable.FindStringSubmatch(token)
		value := values[match[1]]
		switch match[3] {
		case "url":
			return url.QueryEscape(value)
		case "json":
			encoded, _ := json.Marshal(value)
			return string(encoded)
		case "base64":
			return base64.StdEncoding.EncodeToString([]byte(value))
		}
		return value
	})
}

func pointerParts(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	if !strings.HasPrefix(path, "/") {
		return nil, invalid("응답 경로는 /data/count처럼 입력하거나 연결 테스트 결과에서 선택해주세요.")
	}
	parts := strings.Split(path[1:], "/")
	if len(parts) > 32 {
		return nil, invalid("응답 경로는 32단계까지 사용할 수 있습니다.")
	}
	for i, part := range parts {
		for j := 0; j < len(part); j++ {
			if part[j] == '~' {
				if j+1 >= len(part) || (part[j+1] != '0' && part[j+1] != '1') {
					return nil, invalid("응답 경로의 ~ 문자를 ~0으로, / 문자를 ~1로 적어주세요.")
				}
				j++
			}
		}
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts, nil
}
func lookup(root any, path string) (any, bool) {
	parts, err := pointerParts(path)
	if err != nil {
		return nil, false
	}
	value := root
	for _, part := range parts {
		switch node := value.(type) {
		case map[string]any:
			var ok bool
			value, ok = node[part]
			if !ok {
				return nil, false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(node) || strconv.Itoa(index) != part {
				return nil, false
			}
			value = node[index]
		default:
			return nil, false
		}
	}
	return value, value != nil
}

func formatValue(value any, field domain.AddonField) (string, bool) {
	text := ""
	switch v := value.(type) {
	case string:
		text = v
	case json.Number:
		text = string(v)
	case bool:
		text = strconv.FormatBool(v)
	}
	if mapped, ok := field.ValueMap[text]; ok && text != "" {
		return mapped, true
	}
	if field.Format == "count" {
		switch v := value.(type) {
		case []any:
			return strconv.Itoa(len(v)), true
		case map[string]any:
			return strconv.Itoa(len(v)), true
		default:
			return "", false
		}
	}
	if field.Format == "boolean" {
		if b, err := strconv.ParseBool(text); err == nil {
			if b {
				return "예", true
			}
			return "아니요", true
		}
		return "", false
	}
	if field.Format == "text" {
		switch value.(type) {
		case string, json.Number, bool:
			return text, true
		}
		return "", false
	}
	number, err := strconv.ParseFloat(text, 64)
	number *= field.Scale
	if err != nil || math.IsInf(number, 0) || math.IsNaN(number) || math.Abs(number) > 1e15 {
		return "", false
	}
	switch field.Format {
	case "bytes":
		units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
		index := 0
		for math.Abs(number) >= 1024 && index < len(units)-1 {
			number /= 1024
			index++
		}
		return fmt.Sprintf("%.*f %s", field.Decimals, number, units[index]), true
	case "duration":
		seconds := int64(number)
		if seconds < 0 {
			return "", false
		}
		if seconds >= 86400 {
			return fmt.Sprintf("%d일 %d시간", seconds/86400, seconds%86400/3600), true
		}
		if seconds >= 3600 {
			return fmt.Sprintf("%d시간 %d분", seconds/3600, seconds%3600/60), true
		}
		return fmt.Sprintf("%d분 %d초", seconds/60, seconds%60), true
	case "percent":
		return fmt.Sprintf("%.*f%%", field.Decimals, number), true
	default:
		return fmt.Sprintf("%.*f", field.Decimals, number), true
	}
}
