// Package validate_test проверяет контракт практики: Dockerfile и
// docker-compose.yml должны соответствовать требованиям урока 23.
// Этот тест — критерий приёмки: правьте только Dockerfile и docker-compose.yml.
package validate_test

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("чтение %s: %v", path, err)
	}
	return string(data)
}

func lastLineContaining(content, substr string) string {
	last := ""
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, substr) {
			last = line
		}
	}
	return last
}

func TestDockerfile(t *testing.T) {
	content := readFile(t, "../Dockerfile")

	if got := strings.Count(content, "FROM"); got < 2 {
		t.Errorf("Dockerfile: ожидается multi-stage build (два и более FROM), найдено %d", got)
	}
	if !strings.Contains(content, "CGO_ENABLED=0") {
		t.Error("Dockerfile: не задан CGO_ENABLED=0 — бинарь должен быть статическим")
	}

	lastFrom := lastLineContaining(content, "FROM")
	if lastFrom == "" {
		t.Fatal("Dockerfile: не найдено ни одной инструкции FROM")
	}
	if !strings.Contains(lastFrom, "distroless/static") && !strings.Contains(lastFrom, "scratch") {
		t.Errorf("Dockerfile: финальный образ должен быть gcr.io/distroless/static или scratch, а это: %s", strings.TrimSpace(lastFrom))
	}

	if !strings.Contains(content, "EXPOSE 8080") {
		t.Error("Dockerfile: нет EXPOSE 8080")
	}
}

func TestDockerCompose(t *testing.T) {
	content := readFile(t, "../docker-compose.yml")

	var compose struct {
		Services map[string]struct {
			Healthcheck any `yaml:"healthcheck"`
			Volumes     any `yaml:"volumes"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(content), &compose); err != nil {
		t.Fatalf("docker-compose.yml не парсится как YAML: %v", err)
	}

	for _, name := range []string{"orders", "postgres", "redpanda", "redis"} {
		if _, ok := compose.Services[name]; !ok {
			t.Errorf("docker-compose.yml: нет сервиса %q (нужны orders, postgres, redpanda, redis)", name)
		}
	}

	if pg, ok := compose.Services["postgres"]; ok {
		if pg.Healthcheck == nil {
			t.Error("docker-compose.yml: у сервиса postgres нет healthcheck (нужен pg_isready)")
		}
		if pg.Volumes == nil {
			t.Error("docker-compose.yml: у сервиса postgres нет volume — данные не переживут пересоздание контейнера")
		}
	}
	if r, ok := compose.Services["redis"]; ok && r.Healthcheck == nil {
		t.Error("docker-compose.yml: у сервиса redis нет healthcheck (нужен redis-cli ping)")
	}
}
