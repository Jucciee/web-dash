package imports

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"homedashboard/internal/domain"
	"homedashboard/internal/services"
	"homedashboard/internal/storage"
)

type DockerReader interface {
	Containers(context.Context) ([]domain.RunningContainer, error)
}
type Service struct {
	Store             *storage.Repository
	Docker            DockerReader
	ValidateIconAsset func(string) error
}

func (s *Service) Preview(ctx context.Context, userID string, input domain.ImportInput) (domain.ImportPreview, error) {
	files, entry, err := virtualFiles(input.Files, input.EntryPath)
	if err != nil {
		return domain.ImportPreview{}, err
	}
	var candidates []domain.ImportCandidate
	var warnings []string
	switch input.Kind {
	case "nginx":
		candidates, warnings, err = parseNginx(files, entry)
	case "caddy":
		candidates, warnings, err = parseCaddy(files, entry)
	default:
		err = &domain.ValidationError{Message: "Nginx 또는 Caddy 설정을 선택해주세요."}
	}
	if err != nil {
		return domain.ImportPreview{}, err
	}
	name := strings.TrimSpace(input.SourceName)
	if name == "" {
		name = entry
	}
	return s.save(ctx, userID, input.DashboardID, input.Kind, name, candidates, warnings)
}

func (s *Service) ScanDocker(ctx context.Context, userID string, input domain.DockerScanInput) (domain.ImportPreview, error) {
	files, root, err := scanCompose(ctx, input.Folder)
	if err != nil {
		return domain.ImportPreview{}, err
	}
	if len(files) == 0 {
		return domain.ImportPreview{}, &domain.ImportError{Code: "IMPORT_NO_FILES", Message: "이 폴더에서 compose.yml, compose.yaml, docker-compose.yml, docker-compose.yaml을 찾지 못했습니다."}
	}
	host, err := validatePublicHost(input.PublicHost)
	if err != nil {
		return domain.ImportPreview{}, err
	}
	if s.Docker == nil {
		return domain.ImportPreview{}, &domain.ImportError{Code: "DOCKER_UNAVAILABLE", Message: "Docker 연결이 설정되지 않았습니다."}
	}
	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	containers, err := s.Docker.Containers(readCtx)
	if err != nil {
		slog.Warn("docker read failed", "code", "DOCKER_UNAVAILABLE")
		return domain.ImportPreview{}, &domain.ImportError{Code: "DOCKER_UNAVAILABLE", Message: "Docker에 연결하지 못했습니다. Docker Desktop 실행 상태와 서버의 Docker socket 또는 DASHBOARD_DOCKER_HOST 설정을 확인해주세요."}
	}
	candidates, warnings := dockerCandidates(files, containers, host)
	return s.save(ctx, userID, input.DashboardID, "docker", root, candidates, warnings)
}

func (s *Service) save(ctx context.Context, userID, dashboardID, kind, name string, candidates []domain.ImportCandidate, warnings []string) (domain.ImportPreview, error) {
	dashboard, err := s.Store.Dashboard(ctx, dashboardID)
	if err != nil {
		return domain.ImportPreview{}, err
	}
	if len(candidates) > 500 {
		return domain.ImportPreview{}, &domain.ImportError{Code: "IMPORT_LIMIT", Message: "후보가 500개를 초과했습니다. 입력을 나누어 가져와주세요."}
	}
	if candidates == nil {
		candidates = []domain.ImportCandidate{}
	}
	if warnings == nil {
		warnings = []string{}
	}
	existing := map[string]string{}
	for _, group := range dashboard.Groups {
		for _, card := range group.Services {
			existing[storage.CanonicalURL(card.URL)] = card.ID
		}
	}
	seen := map[string]bool{}
	unique := make([]domain.ImportCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		parsedURL, err := services.ValidateURL(candidate.URL)
		if err != nil {
			reason := "접속 주소를 찾지 못했습니다."
			if len(candidate.Warnings) > 0 {
				reason = strings.Join(candidate.Warnings, " ")
			}
			warnings = append(warnings, fmt.Sprintf("%s (%s:%d) 제외: %s", candidate.Name, candidate.File, candidate.Line, reason))
			continue
		}
		candidate.URL = parsedURL
		canonical := storage.CanonicalURL(candidate.URL)
		if candidate.URL != "" && seen[canonical] {
			continue
		}
		seen[canonical] = true
		candidate.ID, candidate.Diff = uuid.NewString(), "new"
		if candidate.URL != "" {
			if id := existing[canonical]; id != "" {
				candidate.Diff, candidate.ExistingServiceID = "duplicate", id
			}
		}
		if candidate.Upstreams == nil {
			candidate.Upstreams = []string{}
		}
		if candidate.Warnings == nil {
			candidate.Warnings = []string{}
		}
		unique = append(unique, candidate)
	}
	sort.SliceStable(unique, func(i, j int) bool {
		return unique[i].File < unique[j].File || (unique[i].File == unique[j].File && unique[i].Line < unique[j].Line)
	})
	preview := domain.ImportPreview{ID: uuid.NewString(), DashboardID: dashboard.ID, Kind: kind, SourceName: name, DashboardRevision: dashboard.Revision, ExpiresAt: time.Now().UTC().Add(30 * time.Minute), Candidates: unique, Warnings: warnings}
	if err := s.Store.SavePreview(ctx, userID, preview); err != nil {
		return domain.ImportPreview{}, err
	}
	slog.Info("import preview created", "kind", kind, "importId", preview.ID, "candidates", len(unique), "warnings", len(warnings))
	return preview, nil
}

func (s *Service) Commit(ctx context.Context, userID, id, key string, input domain.ImportCommit) (domain.ImportResult, error) {
	if _, err := uuid.Parse(key); err != nil {
		return domain.ImportResult{}, &domain.ValidationError{Message: "Idempotency-Key에 UUID를 보내주세요."}
	}
	for i := range input.Items {
		item, err := services.ValidateService(input.Items[i].ServiceInput)
		if err != nil {
			return domain.ImportResult{}, err
		}
		if item.IconMode == "upload" && s.ValidateIconAsset != nil {
			if err := s.ValidateIconAsset(item.IconAssetID); err != nil {
				return domain.ImportResult{}, err
			}
		}
		input.Items[i].ServiceInput = item
	}
	data, _ := json.Marshal(input)
	hash := sha256.Sum256(data)
	result, err := s.Store.CommitImport(ctx, userID, id, key, hex.EncodeToString(hash[:]), input)
	if err == nil {
		slog.Info("import committed", "importId", id, "services", len(result.ServiceIDs))
	}
	return result, err
}
