package main

import (
	"encoding/json"
	"fmt"
	"github.com/bitrise-io/go-utils/log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"time"
)

const domain = "https://api.bitrise.io"
const apiVersion = "v0.1"

// Client Bitrise API client
type Client struct {
	authToken  string
	httpClient http.Client
}

// Artifacts ...
type Build struct {
	Data []struct {
		TriggeredAt                  time.Time   `json:"triggered_at"`
		StartedOnWorkerAt            time.Time   `json:"started_on_worker_at"`
		EnvironmentPrepareFinishedAt time.Time   `json:"environment_prepare_finished_at"`
		FinishedAt                   time.Time   `json:"finished_at"`
		Slug                         string      `json:"slug"`
		Status                       int         `json:"status"`
		StatusText                   string      `json:"status_text"`
		AbortReason                  interface{} `json:"abort_reason"`
		IsOnHold                     bool        `json:"is_on_hold"`
		Branch                       string      `json:"branch"`
		BuildNumber                  int         `json:"build_number"`
		CommitHash                   string      `json:"commit_hash"`
		CommitMessage                string      `json:"commit_message"`
		Tag                          string      `json:"tag"`
		TriggeredWorkflow            string      `json:"triggered_workflow"`
		TriggeredBy                  interface{} `json:"triggered_by"`
		StackConfigType              string      `json:"stack_config_type"`
		StackIdentifier              string      `json:"stack_identifier"`
		OriginalBuildParams          struct {
			Branch                   string `json:"branch"`
			Tag                      string `json:"tag"`
			CommitHash               string `json:"commit_hash"`
			CommitMessage            string `json:"commit_message"`
			WorkflowID               string `json:"workflow_id"`
			BranchDest               string `json:"branch_dest"`
			PullRequestID            string `json:"pull_request_id"`
			PullRequestRepositoryURL string `json:"pull_request_repository_url"`
			PullRequestMergeBranch   string `json:"pull_request_merge_branch"`
			PullRequestHeadBranch    string `json:"pull_request_head_branch"`
			Environments             []struct {
				MappedTo string `json:"mapped_to"`
				Value    string `json:"value"`
				IsExpand bool   `json:"is_expand"`
			} `json:"environments"`
		} `json:"original_build_params"`
		PullRequestID           interface{} `json:"pull_request_id"`
		PullRequestTargetBranch interface{} `json:"pull_request_target_branch"`
		PullRequestViewURL      interface{} `json:"pull_request_view_url"`
		CommitViewURL           interface{} `json:"commit_view_url"`
	} `json:"data"`
	Paging struct {
		TotalItemCount int    `json:"total_item_count"`
		PageItemLimit  int    `json:"page_item_limit"`
		Next           string `json:"next"`
	} `json:"paging"`
}

// Artifact ...
type Logs struct {
	ExpiringRawLogURL     string `json:"expiring_raw_log_url"`
	GeneratedLogChunksNum int    `json:"generated_log_chunks_num"`
	IsArchived            bool   `json:"is_archived"`
	LogChunks             []struct {
		Chunk    string `json:"chunk"`
		Position int    `json:"position"`
	} `json:"log_chunks"`
	Timestamp interface{} `json:"timestamp"`
}

// New Create new Bitrise API client
func New(authToken string) Client {
	return Client{
		authToken:  authToken,
		httpClient: http.Client{Timeout: 20 * time.Second},
	}
}

func (c Client) get(endpoint string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s/%s", domain, apiVersion, endpoint)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &http.Response{}, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("token %s", c.authToken))

	resp, err := c.httpClient.Do(req)
	return resp, err
}

// GetLogForBuild ...
func (c Client) GetBuilds(appSlug, branchName string, workflowName string) (art Build, err error) {
	requestPath := fmt.Sprintf("apps/%s/builds?sort_by=running_first&branch=%s&workflow=%s&status=1&limit=1", appSlug, branchName, workflowName)

	resp, err := c.get(requestPath)
	if err != nil {
		return
	}
	defer responseBodyCloser(resp)

	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		err = fmt.Errorf("failed to get artifacts with status code (%d) for [workflow: %s, branch: %s]", resp.StatusCode, workflowName, branchName)
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&art)
	return
}

// GetLogForBuild ...
func (c Client) GetLogForBuild(appSlug, buildSlug string) (art Logs, err error) {
	requestPath := fmt.Sprintf("apps/%s/builds/%s/log", appSlug, buildSlug)

	resp, err := c.get(requestPath)
	if err != nil {
		return
	}
	defer responseBodyCloser(resp)

	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		err = fmt.Errorf("failed to get artifacts with status code (%d) for [build_slug: %s, app_slug: %s]", resp.StatusCode, appSlug, buildSlug)
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&art)
	return
}

func responseBodyCloser(resp *http.Response) {
	if err := resp.Body.Close(); err != nil {
		log.Printf(" [!] Failed to close response body: %+v", err)
	}
}

func errNoEnv(env string) error {
	return fmt.Errorf("environment variable (%s) is not set", env)
}

func mainE() error {
	accessTokenKey := "API_AUTH_TOKEN"
	accessToken := os.Getenv(accessTokenKey)
	if accessToken == "" {
		return errNoEnv(accessTokenKey)
	}

	appSlugKey := "APP_SLUG"
	appSlug := os.Getenv(appSlugKey)
	if appSlug == "" {
		return errNoEnv(appSlugKey)
	}

	workflowNameKey := "WORKFLOW_NAME"
	workflowName := os.Getenv(workflowNameKey)
	if workflowName == "" {
		return errNoEnv(workflowNameKey)
	}

	branchName := os.Getenv("BITRISE_GIT_BRANCH")

	artifactNameKey := "ARTIFACT_NAME"
	artifactName := os.Getenv(artifactNameKey)
	if artifactName == "" {
		return errNoEnv(artifactNameKey)
	}

	c := New(accessToken)
	builds, err := c.GetBuilds(appSlug, branchName, workflowName)
	if err != nil {
		return err
	}
	buildSlugMap := map[string]string{}
	for _, build := range builds.Data {
		buildSlugMap[workflowName] = build.Slug
	}

	logs, err := c.GetLogForBuild(appSlug, buildSlugMap[workflowName])
	if err != nil {
		return err
	}

	re := regexp.MustCompile(fmt.Sprintf("%s (\\d+.\\d+)+", artifactName))

	for _, line := range logs.LogChunks {
		matches := re.FindStringSubmatch(line.Chunk)
		if len(matches) > 0 {
			log.Infof("Exported environment variable:")
			cmdLog, err := exec.Command("bitrise", "envman", "add", "--key", artifactName, "--value", matches[1]).CombinedOutput()
			if err != nil {
				fmt.Printf("Failed to expose output with envman, error: %#v | output: %s", err, cmdLog)
				os.Exit(1)
			}
			fmt.Printf("- %s: %s\n", artifactName, matches[1])
			break
		}
	}

	//. fmt.Printf("done, [%d byte] downloaded\n", n)

	return nil
}

func main() {
	if err := mainE(); err != nil {
		fmt.Printf("Error: %+v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}
