package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"text/template"

	"github.com/calypr/funnel/api/tes"
)

func TestPublishSelectionRejectsUnsafeFileNames(t *testing.T) {
	for _, path := range []string{"META/Patient\r\nX-Injected: yes.ndjson", "META/../Patient.ndjson", "META/Patient/extra.ndjson", "META/Patient\x00.ndjson"} {
		request, err := json.Marshal(publishRequest{Organization: "org", Project: "project", Revision: strings.Repeat("a", 40), Files: []string{path}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parsePublishRequest(&tes.Task{Tags: map[string]string{publishTag: string(request)}}); err == nil {
			t.Errorf("accepted unsafe metadata filename %q", path)
		}
	}
}

func TestExecutorCredentialsRemainLiteral(t *testing.T) {
	accessToken := "caller-{{ .TaskId }}-\"quoted\""
	githubToken := "github-{{ .TaskId }}"
	input, err := json.Marshal(publishInput{
		ProjectID: "org-project", Organization: "org", Project: "project",
		Repository: "https://github.com/org/repo", Commit: strings.Repeat("a", 40),
		Files: []string{"META/Patient.ndjson"},
	})
	if err != nil {
		t.Fatal(err)
	}
	configured, err := injectExecutorEnvironment("env:\n  # CALYPR_ETL_ENV", accessToken, githubToken, string(input), "http://loom:8080", "http://syfon:8080")
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := template.New("executor").Parse(configured)
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, struct{ TaskId string }{"task-123"}); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	var name string
	for _, line := range strings.Split(rendered.String(), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- name: ") {
			name = strings.TrimPrefix(line, "- name: ")
		} else if strings.HasPrefix(line, "value: ") {
			value := strings.TrimPrefix(line, "value: ")
			if strings.HasPrefix(value, "\"") {
				value, err = strconv.Unquote(value)
				if err != nil {
					t.Fatal(err)
				}
			}
			values[name] = value
		}
	}
	for name, want := range map[string]string{
		"ACCESS_TOKEN": accessToken, "GITHUB_TOKEN": githubToken,
		"INPUT_DATA": string(input), "LOOM_URL": "http://loom:8080",
		"DRS_URL": "http://syfon:8080", "ETL_OPERATION_ID": "task-123",
		"ETL_WORKSPACE_DIR": "/workspace",
	} {
		if values[name] != want {
			t.Errorf("%s changed during executor template rendering", name)
		}
	}
	if strings.Contains(values["INPUT_DATA"], "caller-") || strings.Contains(values["INPUT_DATA"], "github-") {
		t.Fatal("source selection contains credentials")
	}
}
