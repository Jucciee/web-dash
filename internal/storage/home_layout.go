package storage

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"homedashboard/internal/domain"
)

func validateHomeLayout(layout domain.HomeLayout) error {
	invalid := func(message string) error { return &domain.ValidationError{Message: message} }
	if layout.Version != 1 || (layout.Mode != "auto" && layout.Mode != "custom") {
		return invalid("홈 배치의 버전과 모드를 확인해주세요.")
	}
	if layout.Sections == nil || len(layout.Sections) > 60 || (layout.Mode == "auto" && len(layout.Sections) != 0) {
		return invalid("자동 배치는 빈 섹션 목록, 사용자 지정 배치는 최대 60개 섹션으로 설정해주세요.")
	}
	sections, references := map[string]bool{}, map[string]bool{}
	total := 0
	for sectionIndex, section := range layout.Sections {
		sectionKey := strings.ToLower(section.ID)
		if _, err := uuid.Parse(section.ID); err != nil || len(section.ID) != 36 || sections[sectionKey] {
			return invalid("섹션 ID는 중복되지 않는 UUID여야 합니다.")
		}
		sections[sectionKey] = true
		if strings.TrimSpace(section.Name) == "" || utf8.RuneCountInString(section.Name) > 80 || strings.ContainsRune(section.Name, 0) {
			return invalid("섹션 이름은 1~80자로 입력해주세요.")
		}
		if section.Width < 3 || section.Width > 12 || section.MinRows < 0 || section.MinRows > 48 || section.Items == nil {
			return invalid("섹션 너비는 3~12칸, 최소 높이는 자동(0) 또는 1~48칸으로 설정해주세요.")
		}
		if (section.X == nil) != (section.Y == nil) {
			return invalid("섹션의 가로·세로 위치를 함께 설정해주세요.")
		}
		if section.X != nil && (*section.X < 0 || *section.X > 11 || *section.X+section.Width > 12 || *section.Y < 0 || *section.Y > 65535) {
			return invalid("섹션은 홈의 12열 안에 배치해주세요.")
		}
		total += len(section.Items)
		if total > 500 {
			return invalid("홈 배치에는 최대 500개 항목을 넣을 수 있습니다.")
		}
		for index, item := range section.Items {
			switch item.Kind {
			case "link", "addon":
				if _, err := uuid.Parse(item.RefID); err != nil || len(item.RefID) != 36 {
					return invalid("링크와 애드온의 참조 ID를 확인해주세요.")
				}
			case "host", "docker":
				if item.RefID != "" {
					return invalid("호스트와 Docker에는 별도의 참조 ID를 넣지 않습니다.")
				}
			default:
				return invalid("홈 항목은 링크, 애드온, 호스트, Docker 중에서 선택해주세요.")
			}
			key := item.Kind + ":" + strings.ToLower(item.RefID)
			if references[key] {
				return invalid("같은 항목은 홈 배치에 한 번만 넣을 수 있습니다.")
			}
			references[key] = true
			if item.X < 0 || item.X > 11 || item.Width < 1 || item.Width > 12 || item.X+item.Width > 12 || item.Y < 0 || item.Y > 4095 || item.Height < 1 || item.Height > 24 || item.Y+item.Height > 4096 {
				return invalid("항목은 12열 안에 배치하고, 높이는 1~24칸으로 설정해주세요.")
			}
			for _, other := range section.Items[:index] {
				if item.X < other.X+other.Width && item.X+item.Width > other.X && item.Y < other.Y+other.Height && item.Y+item.Height > other.Y {
					return invalid("섹션 안의 항목이 겹칩니다. 배치 편집에서 위치를 조절해주세요.")
				}
			}
		}
		if section.X != nil {
			if *section.Y+homeSectionHeight(section) > 65536 {
				return invalid("섹션 높이가 홈 배치의 범위를 벗어납니다.")
			}
			for _, other := range layout.Sections[:sectionIndex] {
				if other.X != nil && other.Y != nil && *section.X < *other.X+other.Width && *section.X+section.Width > *other.X && *section.Y < *other.Y+homeSectionHeight(other) && *section.Y+homeSectionHeight(section) > *other.Y {
					return invalid("섹션이 겹칩니다. 배치 편집에서 위치를 조절해주세요.")
				}
			}
		}
	}
	return nil
}

func homeSectionHeight(section domain.HomeSection) int {
	rows := max(section.MinRows, 1)
	if len(section.Items) == 0 {
		rows = max(rows, 3)
	}
	for _, item := range section.Items {
		rows = max(rows, item.Y+item.Height)
	}
	return rows + 1 // A heading occupies one grid row.
}
