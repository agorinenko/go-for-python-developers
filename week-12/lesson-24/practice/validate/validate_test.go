// Package validate_test проверяет контракт практики урока 24: k8s-манифесты
// (deployment, service, configmap, hpa) и CI-пайплайн (.github/workflows/ci.yml).
// Этот тест — критерий приёмки: правьте только файлы манифестов и пайплайна.
package validate_test

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("чтение %s: %v", path, err)
	}
	return data
}

func TestDeployment(t *testing.T) {
	var dep struct {
		Spec struct {
			Replicas *int `yaml:"replicas"`
			Template struct {
				Spec struct {
					Containers []struct {
						Name           string `yaml:"name"`
						LivenessProbe  any    `yaml:"livenessProbe"`
						ReadinessProbe any    `yaml:"readinessProbe"`
						StartupProbe   any    `yaml:"startupProbe"`
						Resources      any    `yaml:"resources"`
					} `yaml:"containers"`
				} `yaml:"spec"`
			} `yaml:"template"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(readFile(t, "../deployment.yaml"), &dep); err != nil {
		t.Fatalf("deployment.yaml не парсится как YAML: %v", err)
	}

	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 2 {
		t.Errorf("deployment.yaml: replicas = %v, want 2", dep.Spec.Replicas)
	}

	containers := dep.Spec.Template.Spec.Containers
	if len(containers) == 0 {
		t.Fatal("deployment.yaml: в поде нет ни одного контейнера")
	}
	c := containers[0]

	if c.LivenessProbe == nil {
		t.Error("deployment.yaml: нет livenessProbe")
	}
	if c.ReadinessProbe == nil {
		t.Error("deployment.yaml: нет readinessProbe")
	}
	if c.StartupProbe == nil {
		t.Error("deployment.yaml: нет startupProbe")
	}
	if c.Resources == nil {
		t.Error("deployment.yaml: контейнер без resources — requests/limits обязательны")
	}

	if lp, ok := c.LivenessProbe.(map[string]any); ok {
		httpGet, _ := lp["httpGet"].(map[string]any)
		path, _ := httpGet["path"].(string)
		if path != "/healthz" {
			t.Errorf("deployment.yaml: livenessProbe httpGet path = %q, want /healthz", path)
		}
	} else {
		t.Error("deployment.yaml: livenessProbe должен быть объектом с httpGet")
	}
}

func TestService(t *testing.T) {
	var svc struct {
		Kind string `yaml:"kind"`
		Spec struct {
			Selector any `yaml:"selector"`
			Ports    []struct {
				Port       int `yaml:"port"`
				TargetPort int `yaml:"targetPort"`
			} `yaml:"ports"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(readFile(t, "../service.yaml"), &svc); err != nil {
		t.Fatalf("service.yaml не парсится как YAML: %v", err)
	}
	if svc.Kind != "Service" {
		t.Errorf("service.yaml: kind = %q, want Service", svc.Kind)
	}
	if svc.Spec.Selector == nil {
		t.Error("service.yaml: нет selector (без него Service не выберет поды)")
	}
	if len(svc.Spec.Ports) == 0 {
		t.Error("service.yaml: нет портов")
	} else if p := svc.Spec.Ports[0]; p.Port != 80 || p.TargetPort != 8080 {
		t.Errorf("service.yaml: port %d → targetPort %d, want 80 → 8080", p.Port, p.TargetPort)
	}
}

func TestConfigMap(t *testing.T) {
	var cm struct {
		Kind string            `yaml:"kind"`
		Data map[string]string `yaml:"data"`
	}
	if err := yaml.Unmarshal(readFile(t, "../configmap.yaml"), &cm); err != nil {
		t.Fatalf("configmap.yaml не парсится как YAML: %v", err)
	}
	if cm.Kind != "ConfigMap" {
		t.Errorf("configmap.yaml: kind = %q, want ConfigMap", cm.Kind)
	}
	for _, key := range []string{"HTTP_ADDR", "DB_DSN"} {
		if _, ok := cm.Data[key]; !ok {
			t.Errorf("configmap.yaml: в data нет ключа %q", key)
		}
	}
}

func TestHPA(t *testing.T) {
	var hpa struct {
		Spec struct {
			MinReplicas int `yaml:"minReplicas"`
			MaxReplicas int `yaml:"maxReplicas"`
			Metrics     []struct {
				Type     string `yaml:"type"`
				Resource struct {
					Name   string `yaml:"name"`
					Target struct {
						AverageUtilization int `yaml:"averageUtilization"`
					} `yaml:"target"`
				} `yaml:"resource"`
			} `yaml:"metrics"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(readFile(t, "../hpa.yaml"), &hpa); err != nil {
		t.Fatalf("hpa.yaml не парсится как YAML: %v", err)
	}
	if hpa.Spec.MinReplicas != 2 {
		t.Errorf("hpa.yaml: minReplicas = %d, want 2", hpa.Spec.MinReplicas)
	}
	if hpa.Spec.MaxReplicas != 10 {
		t.Errorf("hpa.yaml: maxReplicas = %d, want 10", hpa.Spec.MaxReplicas)
	}

	found := false
	for _, m := range hpa.Spec.Metrics {
		if m.Type == "Resource" && m.Resource.Name == "cpu" {
			found = true
			if m.Resource.Target.AverageUtilization != 60 {
				t.Errorf("hpa.yaml: averageUtilization = %d, want 60", m.Resource.Target.AverageUtilization)
			}
		}
	}
	if !found {
		t.Error("hpa.yaml: нет CPU-метрики (type: Resource, name: cpu)")
	}
}

func TestCIWorkflow(t *testing.T) {
	var wf struct {
		On   any `yaml:"on"`
		Jobs map[string]struct {
			Steps []struct {
				Name string `yaml:"name"`
				Uses string `yaml:"uses"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(readFile(t, "../.github/workflows/ci.yml"), &wf); err != nil {
		t.Fatalf("ci.yml не парсится как YAML: %v", err)
	}

	// Триггеры: поддерживаем обе формы — on: [push, pull_request] и map-форму.
	hasPush, hasPR := false, false
	switch on := wf.On.(type) {
	case map[string]any:
		_, hasPush = on["push"]
		_, hasPR = on["pull_request"]
	case []any:
		for _, item := range on {
			if s, ok := item.(string); ok {
				hasPush = hasPush || s == "push"
				hasPR = hasPR || s == "pull_request"
			}
		}
	default:
		t.Error("ci.yml: не распознан триггер on (нужны push и pull_request)")
	}
	if !hasPush {
		t.Error("ci.yml: нет триггера on.push")
	}
	if !hasPR {
		t.Error("ci.yml: нет триггера on.pull_request")
	}

	for _, keyword := range []string{"lint", "test", "build", "docker"} {
		found := false
		for _, job := range wf.Jobs {
			for _, step := range job.Steps {
				text := strings.ToLower(step.Name + " " + step.Uses + " " + step.Run)
				if strings.Contains(text, keyword) {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("ci.yml: нет шага %q (проверяется по name/uses/run шагов)", keyword)
		}
	}
}
