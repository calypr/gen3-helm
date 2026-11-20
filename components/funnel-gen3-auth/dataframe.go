package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/calypr/funnel/api/config"
	funnelplugin "github.com/calypr/funnel/api/plugin"
	"github.com/calypr/funnel/api/tes"
)

type dataframePublishRequest struct {
	Organization string `json:"organization"`
	Project      string `json:"project"`
	ExplorerID   string `json:"explorerId"`
	ReceiptID    string `json:"receiptId"`
}

var dataframeReceiptPattern = regexp.MustCompile(`^receipt_[a-f0-9]{64}$`)

func parseDataframePublishRequest(task *tes.Task) (dataframePublishRequest, error) {
	var request dataframePublishRequest
	if err := validateRequestedTask(task); err != nil {
		return request, err
	}
	raw := task.Tags[dataframePublishTag]
	if len(raw) == 0 || len(raw) > 1024 || task.Tags[publishTag] != "" {
		return request, errors.New("Dataframe publish requires exactly one publish type")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, errors.New("Dataframe publish requires organization, project, explorerId, and receiptId")
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return request, errors.New("Dataframe publish tag must contain one JSON object")
	}
	if !projectNamePattern.MatchString(request.Organization) || !projectNamePattern.MatchString(request.Project) ||
		!projectNamePattern.MatchString(request.ExplorerID) || !dataframeReceiptPattern.MatchString(request.ReceiptID) {
		return request, errors.New("Invalid dataframe publish identifiers")
	}
	return request, nil
}

func createDataframeTask(params map[string]string, headers map[string]*funnelplugin.StringList, effective *config.Config, task *tes.Task) *funnelplugin.JobResponse {
	request, err := parseDataframePublishRequest(task)
	if err != nil {
		return response(http.StatusBadRequest, err.Error(), effective, nil)
	}
	if effective.Kubernetes == nil || strings.Count(effective.Kubernetes.ExecutorTemplate, executorMarker) != 1 {
		return response(http.StatusInternalServerError, "Funnel executor template is missing the publish environment marker", effective, nil)
	}
	settings, err := readSettings(params)
	if err != nil {
		return response(http.StatusInternalServerError, "Funnel publish plugin configuration is incomplete", effective, nil)
	}
	workerImage := strings.TrimSpace(params["DATAFRAME_ETL_IMAGE"])
	if workerImage == "" {
		return response(http.StatusInternalServerError, "Funnel dataframe worker image is not configured", effective, nil)
	}
	authorizationHeader, accessToken, err := getAuthorization(headers)
	if err != nil {
		return response(http.StatusUnauthorized, "A bearer authorization token is required", effective, nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	user, status, err := fetchFenceUser(ctx, newUpstreamClient(), settings.fenceURL, authorizationHeader)
	if err != nil {
		return response(upstreamStatus(status), "Could not verify the Gen3 caller", effective, nil)
	}
	if !canCreateProject(user, request.Organization, request.Project) {
		return response(http.StatusForbidden, "Caller cannot create data for this project", effective, nil)
	}

	input, err := json.Marshal(request)
	if err != nil {
		return response(http.StatusInternalServerError, "Could not prepare the dataframe publish task", effective, nil)
	}
	template, err := injectExecutorEnvironment(effective.Kubernetes.ExecutorTemplate, accessToken, "", string(input), settings.loomURL, settings.drsURL, "")
	if err != nil {
		return response(http.StatusInternalServerError, "Could not configure the dataframe publish executor", effective, nil)
	}
	effective.Kubernetes.ExecutorTemplate = template
	fixed := &tes.Task{
		Name:        fmt.Sprintf("Publish dataframe %s/%s", request.Organization, request.Project),
		Description: "Materialize and activate a compiled Loom dataframe",
		Tags:        map[string]string{dataframePublishTag: string(input)},
		Volumes:     []string{"/workspace"},
		Executors:   []*tes.Executor{{Image: workerImage, Command: []string{"/app/funnel-etl", "publish-dataframe"}, Workdir: "/workspace"}},
	}
	return response(http.StatusOK, "", effective, fixed)
}
