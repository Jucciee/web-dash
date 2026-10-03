package imports

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/moby/moby/client"
	"homedashboard/internal/domain"
)

type EngineReader struct{ Host string }

var engineListPath = regexp.MustCompile(`^(/v[0-9]+\.[0-9]+)?/containers/json$`)

func readOnlyDocker(request *http.Request) error {
	path := request.URL.Path
	if (request.Method == http.MethodGet || request.Method == http.MethodHead) && path == "/_ping" {
		return nil
	}
	if request.Method == http.MethodGet && (path == "/version" || engineListPath.MatchString(path)) {
		return nil
	}
	return fmt.Errorf("docker operation is not permitted")
}

func (r EngineReader) Containers(ctx context.Context) ([]domain.RunningContainer, error) {
	opts := []client.Opt{client.WithHostFromEnv(), client.WithTLSClientConfigFromEnv(), client.WithTimeout(8 * time.Second), client.WithHTTPRequestHook(readOnlyDocker)}
	if r.Host != "" {
		opts = append(opts, client.WithHost(r.Host))
	}
	engine, err := client.New(opts...)
	if err != nil {
		return nil, err
	}
	defer engine.Close()
	result, err := engine.ContainerList(ctx, client.ContainerListOptions{})
	if err != nil {
		return nil, err
	}
	containers := []domain.RunningContainer{}
	for _, row := range result.Items {
		if string(row.State) != "running" {
			continue
		}
		labels := row.Labels
		item := domain.RunningContainer{Project: labels["com.docker.compose.project"], Service: labels["com.docker.compose.service"], WorkingDir: labels["com.docker.compose.project.working_dir"], ConfigFiles: labels["com.docker.compose.project.config_files"], PreferredURL: labels["homepage.href"]}
		if len(row.Names) > 0 {
			item.Name = strings.TrimPrefix(row.Names[0], "/")
		}
		for _, port := range row.Ports {
			if port.PublicPort > 0 {
				ip := ""
				if port.IP.IsValid() {
					ip = port.IP.String()
				}
				item.Ports = append(item.Ports, domain.PublishedPort{IP: ip, Public: port.PublicPort, Private: port.PrivatePort, Protocol: port.Type})
			}
		}
		containers = append(containers, item)
	}
	return containers, nil
}
