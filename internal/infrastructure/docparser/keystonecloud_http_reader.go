package docparser

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/justaboyhai-wq/keystone/internal/logger"

	"github.com/justaboyhai-wq/keystone/internal/models/utils"
	"github.com/justaboyhai-wq/keystone/internal/types"
	"github.com/google/uuid"
)

const (
	keystoneCloudReaderBaseURL = "https://keystone.weixin.qq.com/api/v1/doc"
)

// KeystoneCloudSignedDocumentReader implements the docreader HTTP protocol with KeystoneCloud signing.
type KeystoneCloudSignedDocumentReader struct {
	appID               string
	apiKey              string
	client              *http.Client
	initialPollInterval time.Duration
	maxPollInterval     time.Duration
	pollTimeout         time.Duration
}

func NewKeystoneCloudSignedDocumentReader(appID, apiKey string) (*KeystoneCloudSignedDocumentReader, error) {
	if appID == "" {
		return nil, fmt.Errorf("KeystoneCloud appID is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("KeystoneCloud apiKey is required")
	}
	return &KeystoneCloudSignedDocumentReader{
		appID:               appID,
		apiKey:              apiKey,
		initialPollInterval: 500 * time.Millisecond,
		maxPollInterval:     10 * time.Second,
		pollTimeout:         20 * time.Minute,
		client: &http.Client{
			Timeout: 500 * time.Minute,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     90 * time.Second,
				MaxIdleConnsPerHost: 5,
			},
		},
	}, nil
}

func (p *KeystoneCloudSignedDocumentReader) Reconnect(addr string) error {
	return nil
}

func (p *KeystoneCloudSignedDocumentReader) IsConnected() bool { return true }

func (p *KeystoneCloudSignedDocumentReader) ListEngines(ctx context.Context, overrides map[string]string) ([]types.ParserEngineInfo, error) {
	return []types.ParserEngineInfo{{
		Name:        KeystoneCloudEngineName,
		Description: "KeystoneCloud signed docreader",
		FileTypes:   []string{"docx", "doc", "pdf", "md", "markdown", "xlsx", "xls", "pptx", "ppt"},
		Available:   true,
	}}, nil
}

func (p *KeystoneCloudSignedDocumentReader) Read(ctx context.Context, req *types.ReadRequest) (*types.ReadResult, error) {
	logger.Infof(ctx, "[KeystoneCloud] read start file=%q type=%q engine=%q hasURL=%v contentLen=%d requestID=%q",
		req.FileName, req.FileType, req.ParserEngine, strings.TrimSpace(req.URL) != "", len(req.FileContent), req.RequestID)
	body := httpReadRequest{
		FileName:  req.FileName,
		FileType:  req.FileType,
		URL:       req.URL,
		Title:     req.Title,
		RequestID: req.RequestID,
		Config: &httpReadConfig{
			ParserEngine:          req.ParserEngine,
			ParserEngineOverrides: req.ParserEngineOverrides,
		},
	}
	if len(req.FileContent) > 0 {
		body.FileContent = base64.StdEncoding.EncodeToString(req.FileContent)
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		logger.Errorf(context.Background(), "[KeystoneCloud] marshal read request: %v", err)
		return nil, fmt.Errorf("http marshal read request: %w", err)
	}
	httpReq, err := p.newSignedRequest(ctx, http.MethodPost, keystoneCloudReaderBaseURL+"/reader", jsonBody)
	if err != nil {
		logger.Errorf(context.Background(), "[KeystoneCloud] signed read request: %v", err)
		return nil, err
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		logger.Errorf(context.Background(), "[KeystoneCloud] http read request failed: %v", err)
		return nil, fmt.Errorf("http read failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Errorf(context.Background(), "[KeystoneCloud] http read unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("http read status %d: %s", resp.StatusCode, string(bodyBytes))
	}
	var submit keystoneCloudAsyncSubmitResponse
	if err := json.NewDecoder(resp.Body).Decode(&submit); err != nil {
		logger.Errorf(context.Background(), "[KeystoneCloud] decode read submit response: %v", err)
		return nil, fmt.Errorf("http decode read submit response: %w", err)
	}
	if strings.TrimSpace(submit.TaskID) == "" {
		logger.Errorf(context.Background(), "[KeystoneCloud] submit response missing task_id (status=%q message=%q)", submit.Status, submit.Message)
		return nil, fmt.Errorf("keystonecloud docreader submit response missing task_id")
	}
	logger.Infof(ctx, "[KeystoneCloud] task submitted task_id=%s file=%q type=%q", submit.TaskID, req.FileName, req.FileType)
	return p.pollTaskResult(ctx, submit.TaskID)
}

type keystoneCloudAsyncSubmitResponse struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	CreatedAt int64  `json:"created_at"`
}

type keystoneCloudAsyncTaskResponse struct {
	TaskID    string            `json:"task_id"`
	Status    string            `json:"status"`
	Message   string            `json:"message"`
	Progress  float64           `json:"progress"`
	Result    *httpReadResponse `json:"result"`
	Error     string            `json:"error"`
	CreatedAt int64             `json:"created_at"`
	UpdatedAt int64             `json:"updated_at"`
}

func (p *KeystoneCloudSignedDocumentReader) pollTaskResult(ctx context.Context, taskID string) (*types.ReadResult, error) {
	pollCtx := ctx
	if _, ok := ctx.Deadline(); !ok && p.pollTimeout > 0 {
		var cancel context.CancelFunc
		pollCtx, cancel = context.WithTimeout(ctx, p.pollTimeout)
		defer cancel()
	}
	statusURL := keystoneCloudReaderBaseURL + "/" + taskID
	currentInterval := p.initialPollInterval
	for {
		httpReq, err := p.newSignedRequest(pollCtx, http.MethodGet, statusURL, nil)
		if err != nil {
			logger.Errorf(context.Background(), "[KeystoneCloud] poll signed request task_id=%s: %v", taskID, err)
			return nil, err
		}
		resp, err := p.client.Do(httpReq)
		if err != nil {
			logger.Errorf(context.Background(), "[KeystoneCloud] http poll task_id=%s failed: %v", taskID, err)
			return nil, fmt.Errorf("http poll task failed: %w", err)
		}
		var taskResp keystoneCloudAsyncTaskResponse
		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(resp.Body)
				err = fmt.Errorf("http poll task status %d: %s", resp.StatusCode, string(bodyBytes))
				logger.Errorf(context.Background(), "[KeystoneCloud] poll task_id=%s status %d: %s", taskID, resp.StatusCode, string(bodyBytes))
				return
			}
			if decodeErr := json.NewDecoder(resp.Body).Decode(&taskResp); decodeErr != nil {
				err = fmt.Errorf("http decode task response: %w", decodeErr)
				logger.Errorf(context.Background(), "[KeystoneCloud] poll task_id=%s decode response: %v", taskID, decodeErr)
			}
		}()
		if err != nil {
			return nil, err
		}
		switch taskResp.Status {
		case "completed":
			if taskResp.Result == nil {
				logger.Infof(ctx, "[KeystoneCloud] task_id=%s completed with no result payload", taskID)
				return &types.ReadResult{}, nil
			}
			res := fromHTTPReadResponse(taskResp.Result)
			if res.Error != "" {
				logger.Errorf(ctx, "[KeystoneCloud] task_id=%s completed with result.error: %s", taskID, res.Error)
			} else {
				logger.Debugf(ctx, "[KeystoneCloud] task_id=%s completed ok markdownLen=%d", taskID, len(res.MarkdownContent))
			}
			return res, nil
		case "failed":
			if taskResp.Error != "" {
				logger.Errorf(context.Background(), "[KeystoneCloud] task_id=%s failed: %s", taskID, taskResp.Error)
				return nil, fmt.Errorf("keystonecloud docreader task failed: %s", taskResp.Error)
			}
			logger.Errorf(context.Background(), "[KeystoneCloud] task_id=%s failed: %s", taskID, taskResp.Message)
			return nil, fmt.Errorf("keystonecloud docreader task failed: %s", taskResp.Message)
		case "cancelled":
			if taskResp.Error != "" {
				logger.Errorf(context.Background(), "[KeystoneCloud] task_id=%s cancelled: %s", taskID, taskResp.Error)
				return nil, fmt.Errorf("keystonecloud docreader task cancelled: %s", taskResp.Error)
			}
			logger.Errorf(context.Background(), "[KeystoneCloud] task_id=%s cancelled", taskID)
			return nil, fmt.Errorf("keystonecloud docreader task cancelled")
		}
		if err := pollCtx.Err(); err != nil {
			logger.Errorf(ctx, "[KeystoneCloud] poll task_id=%s aborted before sleep: %v", taskID, err)
			return nil, err
		}

		// Exponential backoff: multiply by 1.5 each time, cap at maxPollInterval
		select {
		case <-pollCtx.Done():
			logger.Errorf(ctx, "[KeystoneCloud] poll task_id=%s stopped: %v", taskID, pollCtx.Err())
			return nil, pollCtx.Err()
		case <-time.After(currentInterval):
			// Update interval for next iteration
			nextInterval := time.Duration(float64(currentInterval) * 1.5)
			if nextInterval > p.maxPollInterval {
				nextInterval = p.maxPollInterval
			}
			currentInterval = nextInterval
		}
	}
}

func (p *KeystoneCloudSignedDocumentReader) newSignedRequest(ctx context.Context, method, url string, body []byte) (*http.Request, error) {
	requestID := uuid.New().String()
	if len(body) == 0 {
		body = []byte("{}")
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		logger.Errorf(context.Background(), "[KeystoneCloud] http new request %s %s: %v", method, url, err)
		return nil, fmt.Errorf("http new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.ContentLength = int64(len(body))
	for k, v := range utils.Sign(p.appID, p.apiKey, requestID, string(body)) {
		httpReq.Header.Set(k, v)
	}
	return httpReq, nil
}
