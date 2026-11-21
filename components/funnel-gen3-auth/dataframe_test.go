package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/calypr/funnel/api/config"
	funnelplugin "github.com/calypr/funnel/api/plugin"
	"github.com/calypr/funnel/api/tes"
)

func TestDataframeTaskAcceptsOnlyPinnedPublishInputs(t *testing.T) {
	input := fmt.Sprintf(`{"organization":"HTAN_INT","project":"BForePC","explorerId":"dataframe-default","receiptId":"receipt_%s"}`, strings.Repeat("a", 64))
	task := &tes.Task{Tags: map[string]string{dataframePublishTag: input}}
	request, err := parseDataframePublishRequest(task)
	if err != nil || request.ExplorerID != "dataframe-default" {
		t.Fatalf("parse dataframe task = %+v, %v", request, err)
	}
	task.Executors = []*tes.Executor{{Image: "caller-image"}}
	if _, err = parseDataframePublishRequest(task); err == nil {
		t.Fatal("accepted caller-controlled executor")
	}
	task.Executors = nil
	task.Tags[publishTag] = "{}"
	if _, err = parseDataframePublishRequest(task); err == nil {
		t.Fatal("accepted ambiguous publication type")
	}
}

func TestDataframeTaskUsesCalyprAuthorizationAndFixedWorker(t *testing.T) {
	fence := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/user/" || request.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("unexpected Fence request")
		}
		_, _ = fmt.Fprint(w, `{"sub":"user","resources":["/programs/HTAN_INT","/programs/HTAN_INT/projects"],"authz":{"/programs/HTAN_INT/projects/BForePC":[{"method":"create","service":"*"}]}}`)
	}))
	defer fence.Close()
	settings := map[string]string{"FENCE_URL": fence.URL, "GECKO_URL": "http://gecko", "LOOM_URL": "http://loom", "DRS_URL": "http://syfon", "ETL_IMAGE": "metadata-image", "DATAFRAME_ETL_IMAGE": "operator-image"}
	headers := map[string]*funnelplugin.StringList{"authorization": {Values: []string{"Bearer fixture"}}}
	conf := &config.Config{}
	if err := json.Unmarshal([]byte(`{"kubernetes":{"executorTemplate":"env:\n  # CALYPR_ETL_ENV"}}`), conf); err != nil {
		t.Fatal(err)
	}
	task := &tes.Task{Tags: map[string]string{dataframePublishTag: fmt.Sprintf(`{"organization":"HTAN_INT","project":"BForePC","explorerId":"dataframe-default","receiptId":"receipt_%s"}`, strings.Repeat("a", 64))}}
	got, err := (&Authorizer{}).PluginAction(settings, headers, conf, task, funnelplugin.Type_CREATE)
	if err != nil || got.GetCode() != http.StatusOK {
		t.Fatalf("create response code = %v, message = %s, error = %v", got.GetCode(), got.GetMessage(), err)
	}
	executors := got.GetTask().GetExecutors()
	if len(executors) != 1 || executors[0].Image != "operator-image" || strings.Join(executors[0].Command, " ") != "/app/funnel-etl publish-dataframe" {
		t.Fatal("unexpected executor")
	}
	if strings.Contains(got.GetTask().Tags[dataframePublishTag], "fixture") {
		t.Fatal("authorization token leaked into the task")
	}
	task.Tags[dataframePublishTag] = strings.Replace(task.Tags[dataframePublishTag], "BForePC", "AnotherProject", 1)
	got, err = (&Authorizer{}).PluginAction(settings, headers, conf, task, funnelplugin.Type_CREATE)
	if err != nil || got.GetCode() != http.StatusForbidden {
		t.Fatalf("cross-project request code = %v, error = %v", got.GetCode(), err)
	}
}
