package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/calypr/funnel/api/config"
	funnelplugin "github.com/calypr/funnel/api/plugin"
	"github.com/calypr/funnel/api/tes"
	hplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/protobuf/proto"
)

const (
	publishTag       = "calypr.publish"
	executorMarker   = "# CALYPR_ETL_ENV"
	maxRequestBytes  = 1 << 20
	maxResponseBytes = 1 << 20
	maxMetadataFiles = 2000
	maxFilePathBytes = 255
	requestTimeout   = 15 * time.Second
)

var (
	projectNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)
	commitPattern      = regexp.MustCompile(`^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)
	metadataPattern    = regexp.MustCompile(`^META/[A-Z][A-Za-z0-9]*\.ndjson$`)
)

type Authorizer struct{}

type publishRequest struct {
	Organization string   `json:"organization"`
	Project      string   `json:"project"`
	Revision     string   `json:"revision"`
	Files        []string `json:"files"`
}

type publishInput struct {
	ProjectID    string   `json:"projectId"`
	Organization string   `json:"organization"`
	Project      string   `json:"project"`
	Repository   string   `json:"repository"`
	Commit       string   `json:"commit"`
	Files        []string `json:"files"`
}

type projectConfig struct {
	Organization string `json:"org_title"`
	Project      string `json:"project_title"`
	SourceRepo   string `json:"src_repo"`
}

type authorization struct {
	Method  string `json:"method"`
	Service string `json:"service"`
}

func main() {
	hplugin.Serve(&hplugin.ServeConfig{
		HandshakeConfig: funnelplugin.Handshake,
		Plugins: map[string]hplugin.Plugin{
			"authorize": &funnelplugin.AuthorizePlugin{Impl: &Authorizer{}},
		},
	})
}

func (a *Authorizer) PluginAction(params map[string]string, headers map[string]*funnelplugin.StringList, conf *config.Config, task *tes.Task, action funnelplugin.Type) (*funnelplugin.JobResponse, error) {
	if conf == nil {
		return response(500, "Funnel configuration is unavailable", nil, nil), nil
	}
	effective := proto.Clone(conf).(*config.Config)
	if action != funnelplugin.Type_CREATE {
		if action == funnelplugin.Type_GET || action == funnelplugin.Type_CANCEL {
			return response(200, "", effective, nil), nil
		}
		return response(400, "Unsupported task action", effective, nil), nil
	}

	request, err := parsePublishRequest(task)
	if err != nil {
		return response(400, err.Error(), effective, nil), nil
	}
	if conf.Kubernetes == nil || strings.Count(conf.Kubernetes.ExecutorTemplate, executorMarker) != 1 {
		return response(500, "Funnel executor template is missing the publish environment marker", effective, nil), nil
	}

	settings, err := readSettings(params)
	if err != nil {
		return response(500, "Funnel publish plugin configuration is incomplete", effective, nil), nil
	}
	authorizationHeader, accessToken, err := getAuthorization(headers)
	if err != nil {
		return response(401, "A bearer authorization token is required", effective, nil), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	client := &http.Client{
		Timeout: requestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	user, status, err := fetchFenceUser(ctx, client, settings.fenceURL, authorizationHeader)
	if err != nil {
		return response(upstreamStatus(status), "Could not verify the Gen3 caller", effective, nil), nil
	}
	if !canCreateProject(user, request.Organization, request.Project) {
		return response(403, "Caller cannot create data for this project", effective, nil), nil
	}

	project, status, err := fetchProject(ctx, client, settings.geckoURL, authorizationHeader, request.Organization, request.Project)
	if err != nil {
		return response(upstreamStatus(status), "Could not load the registered project", effective, nil), nil
	}
	owner, repo, err := registeredGitHubRepository(project, request.Organization, request.Project)
	if err != nil {
		return response(403, "The registered project repository is invalid", effective, nil), nil
	}

	githubToken, status, err := exchangeGitHubToken(ctx, client, settings.fenceURL, authorizationHeader, request.Organization, request.Project, owner, repo)
	if err != nil {
		return response(upstreamStatus(status), "Could not obtain a GitHub read token", effective, nil), nil
	}

	input := publishInput{
		ProjectID:    request.Organization + "-" + request.Project,
		Organization: request.Organization,
		Project:      request.Project,
		Repository:   "https://github.com/" + owner + "/" + repo,
		Commit:       strings.ToLower(request.Revision),
		Files:        request.Files,
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return response(500, "Could not prepare the publish task", effective, nil), nil
	}

	injected, err := injectExecutorEnvironment(conf.Kubernetes.ExecutorTemplate, accessToken, githubToken, string(inputJSON), settings.loomURL, settings.drsURL)
	if err != nil {
		return response(500, "Could not configure the publish executor", effective, nil), nil
	}
	effective.Kubernetes.ExecutorTemplate = injected

	canonicalTag, err := json.Marshal(request)
	if err != nil {
		return response(500, "Could not prepare the publish task", effective, nil), nil
	}
	fixedTask := &tes.Task{
		Name:        fmt.Sprintf("Publish %s/%s at %.12s", request.Organization, request.Project, request.Revision),
		Description: "Publish selected metadata files",
		Tags:        map[string]string{publishTag: string(canonicalTag)},
		Volumes:     []string{"/workspace"},
		Executors: []*tes.Executor{{
			Image:   settings.etlImage,
			Command: []string{"/app/funnel-etl", "run"},
			Workdir: "/workspace",
		}},
	}
	return response(200, "", effective, fixedTask), nil
}

type pluginSettings struct {
	fenceURL string
	geckoURL string
	loomURL  string
	drsURL   string
	etlImage string
}

func readSettings(params map[string]string) (pluginSettings, error) {
	var settings pluginSettings
	var err error
	if settings.fenceURL, err = serviceURL(params["FENCE_URL"]); err != nil {
		return pluginSettings{}, err
	}
	if settings.geckoURL, err = serviceURL(params["GECKO_URL"]); err != nil {
		return pluginSettings{}, err
	}
	if settings.loomURL, err = serviceURL(params["LOOM_URL"]); err != nil {
		return pluginSettings{}, err
	}
	if settings.drsURL, err = serviceURL(params["DRS_URL"]); err != nil {
		return pluginSettings{}, err
	}
	settings.etlImage = strings.TrimSpace(params["ETL_IMAGE"])
	if settings.etlImage == "" || strings.ContainsAny(settings.etlImage, " \t\r\n") {
		return pluginSettings{}, errors.New("invalid ETL image")
	}
	return settings, nil
}

func serviceURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("invalid trusted service URL")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func parsePublishRequest(task *tes.Task) (publishRequest, error) {
	if task == nil {
		return publishRequest{}, errors.New("Publish task is required")
	}
	if len(task.Executors) != 0 || len(task.Inputs) != 0 || len(task.Outputs) != 0 || task.Resources != nil || len(task.Volumes) != 0 {
		return publishRequest{}, errors.New("Publish tasks cannot supply executors, inputs, outputs, resources, or volumes")
	}
	if _, exists := task.Tags["_WORKER_SA"]; exists {
		return publishRequest{}, errors.New("Publish tasks cannot select a worker service account")
	}
	raw, ok := task.Tags[publishTag]
	if !ok || len(raw) == 0 || len(raw) > maxRequestBytes {
		return publishRequest{}, errors.New("Publish tag is missing or too large")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var request publishRequest
	if err := decoder.Decode(&request); err != nil {
		return publishRequest{}, errors.New("Publish tag must contain organization, project, revision, and files")
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return publishRequest{}, errors.New("Publish tag must contain one JSON object")
	}
	if !projectNamePattern.MatchString(request.Organization) || request.Organization == "." || request.Organization == ".." || !projectNamePattern.MatchString(request.Project) || request.Project == "." || request.Project == ".." {
		return publishRequest{}, errors.New("Organization and project must be valid Gen3 identifiers")
	}
	if !commitPattern.MatchString(request.Revision) {
		return publishRequest{}, errors.New("Revision must be a full 40- or 64-character commit SHA")
	}
	if len(request.Files) == 0 || len(request.Files) > maxMetadataFiles {
		return publishRequest{}, errors.New("At least one metadata file is required")
	}
	seen := make(map[string]struct{}, len(request.Files))
	for _, path := range request.Files {
		if len(path) > maxFilePathBytes || !metadataPattern.MatchString(path) || strings.ContainsAny(path, "\\\x00\r\n\t") {
			return publishRequest{}, errors.New("Files must be direct META/*.ndjson paths")
		}
		name := strings.TrimPrefix(path, "META/")
		if name == "." || name == ".." || strings.TrimSpace(name) != name {
			return publishRequest{}, errors.New("Files must be direct META/*.ndjson paths")
		}
		if _, exists := seen[path]; exists {
			return publishRequest{}, errors.New("Metadata file paths must be unique")
		}
		seen[path] = struct{}{}
	}
	request.Revision = strings.ToLower(request.Revision)
	sort.Strings(request.Files)
	return request, nil
}

func getAuthorization(headers map[string]*funnelplugin.StringList) (string, string, error) {
	var raw string
	for key, value := range headers {
		if strings.EqualFold(key, "authorization") && value != nil {
			values := value.GetValues()
			if len(values) != 1 || raw != "" {
				return "", "", errors.New("ambiguous Authorization header")
			}
			raw = strings.TrimSpace(values[0])
		}
	}
	parts := strings.Fields(raw)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || parts[1] == "" {
		return "", "", errors.New("missing bearer token")
	}
	return raw, parts[1], nil
}

func fetchFenceUser(ctx context.Context, client *http.Client, fenceURL, authorizationHeader string) (map[string]json.RawMessage, int, error) {
	endpoint, err := url.JoinPath(fenceURL, "user")
	if err != nil {
		return nil, 0, err
	}
	endpoint += "/"
	body, status, err := requestJSON(ctx, client, http.MethodGet, endpoint, authorizationHeader, nil)
	if err != nil {
		return nil, status, err
	}
	var user map[string]json.RawMessage
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, status, errors.New("invalid Fence user response")
	}
	return user, status, nil
}

func canCreateProject(user map[string]json.RawMessage, organization, project string) bool {
	var resources []string
	if json.Unmarshal(user["resources"], &resources) != nil {
		return false
	}
	resourceSet := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		resourceSet[resource] = struct{}{}
	}
	if _, ok := resourceSet["/programs/"+organization]; !ok {
		return false
	}
	if _, ok := resourceSet["/programs/"+organization+"/projects"]; !ok {
		return false
	}
	var authz map[string][]authorization
	if json.Unmarshal(user["authz"], &authz) != nil {
		return false
	}
	for _, permission := range authz["/programs/"+organization+"/projects/"+project] {
		if permission.Method == "create" && permission.Service == "*" {
			return true
		}
	}
	return false
}

func fetchProject(ctx context.Context, client *http.Client, geckoURL, authorizationHeader, organization, project string) (projectConfig, int, error) {
	endpoint, err := url.JoinPath(geckoURL, "config", "projects", organization, project)
	if err != nil {
		return projectConfig{}, 0, err
	}
	body, status, err := requestJSON(ctx, client, http.MethodGet, endpoint, authorizationHeader, nil)
	if err != nil {
		return projectConfig{}, status, err
	}
	var registered projectConfig
	if err := json.Unmarshal(body, &registered); err != nil {
		return projectConfig{}, status, errors.New("invalid Gecko project response")
	}
	return registered, status, nil
}

func registeredGitHubRepository(project projectConfig, organization, name string) (string, string, error) {
	if project.Organization != organization || project.Project != name {
		return "", "", errors.New("Gecko project identity does not match request")
	}
	raw := strings.TrimSpace(project.SourceRepo)
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Host, "github.com") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", errors.New("Gecko repository must use github.com")
	}
	repositoryPath := strings.Trim(strings.TrimSpace(parsed.Path), "/")
	repositoryPath = strings.TrimSuffix(repositoryPath, ".git")
	parts := strings.Split(repositoryPath, "/")
	if len(parts) != 2 || !githubSegment(parts[0]) || !githubSegment(parts[1]) {
		return "", "", errors.New("invalid Gecko GitHub repository")
	}
	return parts[0], parts[1], nil
}

func githubSegment(value string) bool {
	if value == "" || value == "." || value == ".." || len(value) > 100 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune("-_.", char) {
			continue
		}
		return false
	}
	return true
}

func exchangeGitHubToken(ctx context.Context, client *http.Client, fenceURL, authorizationHeader, organization, project, owner, repo string) (string, int, error) {
	endpoint, err := url.JoinPath(fenceURL, "credentials", "github")
	if err != nil {
		return "", 0, err
	}
	requestBody, err := json.Marshal(map[string]string{
		"action":       "installation_token",
		"organization": organization,
		"project":      project,
		"owner":        owner,
		"repo":         repo,
		"access":       "read",
	})
	if err != nil {
		return "", 0, err
	}
	body, status, err := requestJSON(ctx, client, http.MethodPost, endpoint, authorizationHeader, requestBody)
	if err != nil {
		return "", status, err
	}
	var result struct {
		Token      string    `json:"token"`
		ExpiresAt  time.Time `json:"expires_at"`
		Repository struct {
			Owner string `json:"owner"`
			Repo  string `json:"repo"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.Token == "" || len(result.Token) > 4096 || strings.ContainsAny(result.Token, " \t\r\n") {
		return "", status, errors.New("invalid Fence token response")
	}
	if !strings.EqualFold(result.Repository.Owner, owner) || !strings.EqualFold(result.Repository.Repo, repo) {
		return "", status, errors.New("Fence returned a token for another repository")
	}
	if result.ExpiresAt.IsZero() || time.Until(result.ExpiresAt) < time.Minute {
		return "", status, errors.New("Fence token is expired or near expiry")
	}
	return result.Token, status, nil
}

func requestJSON(ctx context.Context, client *http.Client, method, endpoint, authorizationHeader string, body []byte) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", authorizationHeader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	limited, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(limited) > maxResponseBytes {
		return nil, response.StatusCode, errors.New("invalid upstream response body")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, response.StatusCode, errors.New("upstream request was rejected")
	}
	return limited, response.StatusCode, nil
}

func upstreamStatus(status int) int64 {
	if status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusNotFound {
		return int64(status)
	}
	if status >= 400 && status < 500 {
		return int64(status)
	}
	return http.StatusBadGateway
}

func injectExecutorEnvironment(template, accessToken, githubToken, inputJSON, loomURL, drsURL string) (string, error) {
	lines := strings.Split(template, "\n")
	markerLine := -1
	indent := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == executorMarker {
			if markerLine != -1 {
				return "", errors.New("executor marker must be unique")
			}
			markerLine = i
			indent = line[:strings.Index(line, executorMarker)]
		}
	}
	if markerLine == -1 {
		return "", errors.New("executor marker is missing")
	}
	entries := []string{
		"- name: ACCESS_TOKEN",
		"  value: {{printf \"%q\" " + strconv.Quote(accessToken) + "}}",
		"- name: GITHUB_TOKEN",
		"  value: {{printf \"%q\" " + strconv.Quote(githubToken) + "}}",
		"- name: INPUT_DATA",
		"  value: {{printf \"%q\" " + strconv.Quote(inputJSON) + "}}",
		"- name: LOOM_URL",
		"  value: {{printf \"%q\" " + strconv.Quote(loomURL) + "}}",
		"- name: DRS_URL",
		"  value: {{printf \"%q\" " + strconv.Quote(drsURL) + "}}",
		"- name: ETL_WORKSPACE_DIR",
		"  value: /workspace",
		"- name: ETL_OPERATION_ID",
		"  value: {{printf \"%q\" .TaskId}}",
	}
	for i := range entries {
		entries[i] = indent + entries[i]
	}
	lines[markerLine] = strings.Join(entries, "\n")
	return strings.Join(lines, "\n"), nil
}

func response(code int64, message string, conf *config.Config, task *tes.Task) *funnelplugin.JobResponse {
	return &funnelplugin.JobResponse{Code: code, Message: message, Config: conf, Task: task}
}
