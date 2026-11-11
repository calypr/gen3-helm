package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"text/template"

	funnelplugin "github.com/calypr/funnel/api/plugin"
	"github.com/calypr/funnel/api/tes"
)

func TestAuthenticateUsesFenceSubject(t *testing.T) {
	fence := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/user/" || req.Header.Get("Cookie") != "" {
			t.Error("unexpected Fence user request")
		}
		if req.Header.Get("Authorization") != "Bearer fixture" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"sub":"42","name":"alice","is_admin":true}`))
	}))
	defer fence.Close()
	plugin := &Authorizer{}
	params := map[string]string{"FENCE_URL": fence.URL}
	for _, tc := range []struct {
		header  string
		code    int64
		subject string
	}{
		{"Bearer fixture", http.StatusOK, "42"},
		{"Bearer forged", http.StatusUnauthorized, ""},
		{"", http.StatusUnauthorized, ""},
	} {
		headers := map[string]*funnelplugin.StringList{"authorization": {Values: []string{tc.header}}}
		got, err := plugin.PluginAction(params, headers, nil, nil, funnelplugin.Type_AUTHENTICATE)
		if err != nil || got.GetCode() != tc.code || got.GetUserId() != tc.subject || got.GetConfig() != nil {
			t.Errorf("authorization result = %v, %v", got, err)
		}
	}
}

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
