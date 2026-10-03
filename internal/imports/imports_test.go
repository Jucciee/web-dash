package imports

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"homedashboard/internal/domain"
	"homedashboard/internal/storage"
)

func TestProxyFileBoundariesAndCandidates(t *testing.T) {
	nginx := map[string]string{"nginx.conf": "http { include sites/*.conf; }", "sites/photos.conf": "server { listen 443 ssl; server_name photos.example.com; location / { proxy_pass http://photos:3000; } }"}
	cards, _, err := parseNginx(nginx, "nginx.conf")
	if err != nil || len(cards) != 1 || cards[0].URL != "https://photos.example.com" || cards[0].Name != "photos" {
		t.Fatalf("nginx candidates: %+v, %v", cards, err)
	}
	if len(cards[0].Upstreams) != 1 || cards[0].Upstreams[0] != "http://photos:3000" {
		t.Fatalf("missing nginx upstream: %+v", cards)
	}
	pathCards, _, err := parseNginx(map[string]string{"nginx.conf": "server { listen 8088; server_name lab.example.com; location /photos/ { proxy_pass http://photos:3000; } location ~ /dynamic { proxy_pass http://dynamic:3000; } }"}, "nginx.conf")
	if err != nil || len(pathCards) != 2 || pathCards[0].URL != "http://lab.example.com:8088/photos/" || pathCards[1].URL != "" {
		t.Fatalf("nginx location paths: %+v, %v", pathCards, err)
	}
	if _, _, err := parseNginx(map[string]string{"nginx.conf": "include /etc/passwd;"}, "nginx.conf"); err == nil {
		t.Fatal("include must not read server files")
	}
	if _, _, err := virtualFiles([]domain.ImportFile{{Path: "../secret", Content: "secret"}}, "../secret"); err == nil {
		t.Fatal("virtual traversal was accepted")
	}
	if _, _, err := parseNginx(map[string]string{"nginx.conf": "server { listen 80;"}, "nginx.conf"); err == nil {
		t.Fatal("unclosed nginx block was accepted")
	}
	caddy := map[string]string{"Caddyfile": "{\n email owner@example.com\n}\n(proxy) {\n reverse_proxy photos:3000\n}\nphotos.example.com {\n import proxy\n}\nhttp://files.example.com:8080 {\n reverse_proxy files:8080\n}\n*.example.com {\n respond ok\n}"}
	cards, _, err = parseCaddy(caddy, "Caddyfile")
	if err != nil || len(cards) != 3 || cards[0].URL != "https://photos.example.com" || cards[1].URL != "http://files.example.com:8080" || cards[2].URL != "" {
		t.Fatalf("caddy candidates: %+v, %v", cards, err)
	}
	if len(cards[0].Upstreams) != 1 || cards[0].Upstreams[0] != "photos:3000" {
		t.Fatalf("snippet not resolved: %+v", cards[0])
	}
	cards, _, err = parseCaddy(map[string]string{"Caddyfile": "import sites/*", "sites/media": "media.example.com {\n reverse_proxy media:8096\n}"}, "Caddyfile")
	if err != nil || len(cards) != 1 || cards[0].URL != "https://media.example.com" {
		t.Fatalf("caddy virtual import: %+v, %v", cards, err)
	}
	for _, invalid := range []string{"import /etc/passwd", "import missing", "import Caddyfile", "photos.example.com {"} {
		if _, _, err := parseCaddy(map[string]string{"Caddyfile": invalid}, "Caddyfile"); err == nil {
			t.Fatalf("unsafe/invalid caddy input accepted: %s", invalid)
		}
	}
}

func TestDockerScanUsesRuntimePortsAndProjectPaths(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "photos")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(project, "compose.yml")
	if err := os.WriteFile(filename, []byte("services:\n  photos:\n    image: example/photos\n    ports: [\"9999:3000\"]\n  database:\n    image: postgres\n"), 0600); err != nil {
		t.Fatal(err)
	}
	files, _, err := scanCompose(context.Background(), root)
	if err != nil || len(files) != 1 {
		t.Fatalf("scan: %+v, %v", files, err)
	}
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_ping" {
			w.Header().Set("API-Version", "1.56")
			_, _ = w.Write([]byte("OK"))
			return
		}
		if !engineListPath.MatchString(r.URL.Path) || r.Method != "GET" {
			t.Errorf("unexpected Docker operation: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not allowed", 403)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"Id": strings.Repeat("a", 64), "Names": []string{"/photos"}, "State": "running", "Labels": map[string]string{"com.docker.compose.project": "photos", "com.docker.compose.service": "photos", "com.docker.compose.project.config_files": filename}, "Ports": []map[string]any{{"IP": "0.0.0.0", "PublicPort": 31080, "PrivatePort": 3000, "Type": "tcp"}, {"IP": "::", "PublicPort": 31080, "PrivatePort": 3000, "Type": "tcp"}}},
			{"Id": strings.Repeat("b", 64), "Names": []string{"/unrelated"}, "State": "running", "Labels": map[string]string{"com.docker.compose.project": "photos", "com.docker.compose.service": "photos", "com.docker.compose.project.config_files": filepath.Join(root, "elsewhere", "compose.yml")}, "Ports": []map[string]any{{"IP": "0.0.0.0", "PublicPort": 9999, "PrivatePort": 3000, "Type": "tcp"}}},
			{"Id": strings.Repeat("c", 64), "State": "exited", "Labels": map[string]string{"com.docker.compose.project": "photos", "com.docker.compose.service": "database", "com.docker.compose.project.config_files": filename}},
		})
	}))
	defer engine.Close()
	containers, err := (EngineReader{Host: "tcp://" + strings.TrimPrefix(engine.URL, "http://")}).Containers(context.Background())
	if err != nil || len(containers) != 2 {
		t.Fatalf("engine read: %+v, %v", containers, err)
	}
	cards, warnings := dockerCandidates(files, containers, "192.168.1.10")
	if len(cards) != 2 || cards[0].URL != "http://192.168.1.10:31080" || cards[1].URL != cards[0].URL {
		t.Fatalf("must use actual published ports, not YAML port: %+v", cards)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "database") {
		t.Fatalf("stopped service should be warning: %+v", warnings)
	}
	if dockerPath("/run/desktop/mnt/host/d/Docker/photos/compose.yml") != dockerPath("D:\\Docker\\photos\\compose.yml") {
		t.Fatal("Docker Desktop Windows path mapping failed")
	}
	for _, name := range []string{"start", "stop", "restart"} {
		r, _ := http.NewRequest("POST", engine.URL+"/v1.56/containers/test/"+name, nil)
		if err := readOnlyDocker(r); err == nil {
			t.Fatalf("Docker mutation accepted: %s", name)
		}
	}
	if _, _, err := scanCompose(context.Background(), "relative/folder"); err == nil {
		t.Fatal("relative server folder accepted")
	}
}

func TestImportCommitAtomicIdempotentDuplicateSafe(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	defer pool.Close()
	store := &storage.Repository{DB: db}
	ctx := context.Background()
	user, err := store.Setup(ctx, "owner", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	list, _ := store.Dashboards(ctx)
	dashboard, _ := store.Dashboard(ctx, list[0].ID)
	svc := &Service{Store: store}
	preview, err := svc.Preview(ctx, user.ID, domain.ImportInput{DashboardID: dashboard.ID, Kind: "caddy", EntryPath: "Caddyfile", Files: []domain.ImportFile{{Path: "Caddyfile", Content: "photos.example.com { reverse_proxy photos:3000 }\nfiles.example.com { reverse_proxy files:8080 }\n:8080 { reverse_proxy missing:3000 }"}}})
	if err != nil || len(preview.Candidates) != 2 {
		t.Fatalf("preview: %+v, %v", preview, err)
	}
	if len(preview.Warnings) == 0 || !strings.Contains(strings.Join(preview.Warnings, " "), "제외") {
		t.Fatal("unresolved candidate was not explained as excluded")
	}
	for _, candidate := range preview.Candidates {
		if candidate.URL == "" {
			t.Fatal("unresolved candidate remained in preview")
		}
	}
	input := domain.ImportCommit{DashboardRevision: dashboard.Revision}
	for _, card := range preview.Candidates {
		input.Items = append(input.Items, domain.ImportSelection{CandidateID: card.ID, ServiceInput: domain.ServiceInput{GroupID: dashboard.Groups[0].ID, Name: card.Name, URL: card.URL, IconKey: "server"}})
	}
	key := uuid.NewString()
	input.Items[1].GroupID = uuid.NewString()
	if _, err := svc.Commit(ctx, user.ID, preview.ID, key, input); err == nil {
		t.Fatal("invalid group accepted")
	}
	after, _ := store.Dashboard(ctx, dashboard.ID)
	if len(after.Groups[0].Services) != 0 || after.Revision != dashboard.Revision {
		t.Fatal("failed import wrote partial cards")
	}
	input.Items[1].GroupID = dashboard.Groups[0].ID
	result, err := svc.Commit(ctx, user.ID, preview.ID, key, input)
	if err != nil || len(result.ServiceIDs) != 2 {
		t.Fatalf("commit: %+v, %v", result, err)
	}
	replay, err := svc.Commit(ctx, user.ID, preview.ID, key, input)
	if err != nil || strings.Join(replay.ServiceIDs, ",") != strings.Join(result.ServiceIDs, ",") {
		t.Fatalf("idempotent replay: %+v, %v", replay, err)
	}
	input.Items[0].Name = "changed replay"
	if _, err := svc.Commit(ctx, user.ID, preview.ID, key, input); err == nil {
		t.Fatal("same key different content accepted")
	}
	preview, err = svc.Preview(ctx, user.ID, domain.ImportInput{DashboardID: dashboard.ID, Kind: "nginx", EntryPath: "nginx.conf", Files: []domain.ImportFile{{Path: "nginx.conf", Content: "server { listen 443 ssl; server_name photos.example.com; }"}}})
	if err != nil || preview.Candidates[0].Diff != "duplicate" {
		t.Fatalf("repeat import must detect existing URL: %+v, %v", preview, err)
	}
	if _, err := store.Preview(ctx, uuid.NewString(), preview.ID); err == nil {
		t.Fatal("another user read preview")
	}
	_, err = store.CreateGroup(ctx, dashboard.ID, "new group")
	if err != nil {
		t.Fatal(err)
	}
	input = domain.ImportCommit{DashboardRevision: preview.DashboardRevision, Items: []domain.ImportSelection{{CandidateID: preview.Candidates[0].ID, ServiceInput: domain.ServiceInput{GroupID: dashboard.Groups[0].ID, Name: "photos", URL: "https://new.example.com"}}}}
	if _, err := svc.Commit(ctx, user.ID, preview.ID, uuid.NewString(), input); err != domain.ErrConflict {
		t.Fatalf("stale dashboard must conflict: %v", err)
	}
}
