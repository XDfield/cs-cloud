package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"cs-cloud/internal/logger"
)

var portPatterns = []*regexp.Regexp{
	regexp.MustCompile(`listening on http://[\d.:]+:(\d+)`),
	regexp.MustCompile(`internal server on port (\d+)`),
	regexp.MustCompile(`server listening on .+:(\d+)`),
}

const OpenCodeCLIBinary = "cs"

type OpenCodeAgent struct {
	mu    sync.Mutex
	id    string
	state AgentState

	cliPath  string
	workDir  string
	customEnv map[string]string
	endpoint string
	cmd     *exec.Cmd
	waitCh  chan error
	cancel  context.CancelFunc

	sessionID    string
	modelInfo    *ModelInfo
	eventEmitter func(Event)

	httpClient *http.Client
}

func NewOpenCodeAgent(cfg AgentConfig) *OpenCodeAgent {
	cliPath := OpenCodeCLIBinary
	if extra := cfg.Extra; extra != nil {
		if p, ok := extra["cli_path"].(string); ok && p != "" {
			cliPath = p
		}
	}
	return &OpenCodeAgent{
		id:         cfg.ID,
		cliPath:    cliPath,
		workDir:    cfg.WorkingDir,
		customEnv:  cfg.CustomEnv,
		state:      StateIdle,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

func (a *OpenCodeAgent) ID() string       { return a.id }
func (a *OpenCodeAgent) Backend() string   { return "opencode" }
func (a *OpenCodeAgent) Driver() string    { return "http" }
func (a *OpenCodeAgent) PID() int {
	if a.cmd != nil && a.cmd.Process != nil {
		return a.cmd.Process.Pid
	}
	return 0
}
func (a *OpenCodeAgent) SessionID() string { return a.sessionID }
func (a *OpenCodeAgent) Endpoint() string  { return a.endpoint }
func (a *OpenCodeAgent) State() AgentState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state
}

func (a *OpenCodeAgent) SetEventEmitter(emitter func(Event)) {
	a.eventEmitter = emitter
}

func (a *OpenCodeAgent) setState(s AgentState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = s
}

func (a *OpenCodeAgent) emit(event Event) {
	if a.eventEmitter != nil {
		a.eventEmitter(event)
	}
}

func (a *OpenCodeAgent) Start(ctx context.Context) error {
	a.setState(StateConnecting)

	agentCtx, agentCancel := context.WithCancel(ctx)
	a.cancel = agentCancel

	endpoint, err := a.spawnAndWaitForPort(agentCtx)
	if err != nil {
		a.setState(StateError)
		a.cancel = nil
		agentCancel()
		return fmt.Errorf("spawn opencode: %w", err)
	}
	a.endpoint = endpoint
	logger.Info("opencode endpoint resolved: %s", a.endpoint)

	resp, err := a.doGet(agentCtx, "/global/health")
	if err != nil {
		a.setState(StateError)
		a.Kill()
		return fmt.Errorf("opencode health check failed: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.setState(StateError)
		a.Kill()
		return fmt.Errorf("opencode health check returned status %d", resp.StatusCode)
	}

	a.setState(StateConnected)

	go a.subscribeEvents(agentCtx)

	return nil
}

func (a *OpenCodeAgent) Kill() error {
	a.setState(StateDisconnected)

	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}

	if a.cmd != nil && a.cmd.Process != nil {
		a.gracefulShutdown(5 * time.Second)
	}
	if a.httpClient != nil {
		a.httpClient.CloseIdleConnections()
	}
	return nil
}

func (a *OpenCodeAgent) gracefulShutdown(timeout time.Duration) {
	if a.endpoint != "" && a.httpClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint+"/global/dispose", nil)
		if req != nil {
			resp, err := a.httpClient.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}
		cancel()
	}

	signalTerminate(a.cmd.Process.Pid)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if a.waitCh == nil {
			return
		}
		select {
		case <-a.waitCh:
			a.waitCh = nil
			return
		case <-time.After(100 * time.Millisecond):
		}
	}

	killProcessTree(a.cmd.Process.Pid)
	if a.waitCh != nil {
		<-a.waitCh
		a.waitCh = nil
	}
}

func (a *OpenCodeAgent) SendMessage(ctx context.Context, msg PromptMessage) error {
	if a.sessionID == "" {
		return fmt.Errorf("no active session")
	}
	body := map[string]any{
		"content": msg.Content,
		"files":   msg.Files,
	}
	_, err := a.doPost(ctx, "/session/"+a.sessionID+"/prompt_async", body)
	if err != nil {
		return fmt.Errorf("send prompt failed: %w", err)
	}
	return nil
}

func (a *OpenCodeAgent) CancelPrompt(ctx context.Context) error {
	if a.sessionID == "" {
		return fmt.Errorf("no active session")
	}
	_, err := a.doPost(ctx, "/session/"+a.sessionID+"/abort", nil)
	return err
}

func (a *OpenCodeAgent) ConfirmPermission(ctx context.Context, callID string, optionID string) error {
	if a.sessionID == "" {
		return fmt.Errorf("no active session")
	}
	body := map[string]any{"response": optionID}
	_, err := a.doPost(ctx, "/session/"+a.sessionID+"/permissions/"+callID, body)
	return err
}

func (a *OpenCodeAgent) PendingPermissions() []PermissionInfo { return nil }

func (a *OpenCodeAgent) GetModelInfo() *ModelInfo { return a.modelInfo }

func (a *OpenCodeAgent) SetModel(ctx context.Context, modelID string) (*ModelInfo, error) {
	return a.modelInfo, nil
}

func (a *OpenCodeAgent) spawnAndWaitForPort(ctx context.Context) (string, error) {
	cliName := a.cliPath

	cmd := exec.CommandContext(ctx, cliName, "serve")
	if a.workDir != "" {
		cmd.Dir = a.workDir
	}
	env := append(os.Environ(), "OPENCODE_DISABLE_EMBEDDED_WEB_UI=1")
	for k, v := range a.customEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	setCmdProcessGroup(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start %s serve: %w", cliName, err)
	}

	a.cmd = cmd
	a.waitCh = make(chan error, 1)
	go func() { a.waitCh <- cmd.Wait() }()

	endpointCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			logger.Debug("opencode stdout: %s", line)
			for _, pat := range portPatterns {
				matches := pat.FindStringSubmatch(line)
				if len(matches) >= 2 {
					endpointCh <- "http://127.0.0.1:" + matches[1]
					return
				}
			}
		}
		errCh <- fmt.Errorf("opencode process exited before printing port")
	}()

	timeout := time.After(30 * time.Second)
	select {
	case ep := <-endpointCh:
		return ep, nil
	case err := <-errCh:
		return "", err
	case <-a.waitCh:
		return "", fmt.Errorf("opencode process exited unexpectedly")
	case <-timeout:
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("timeout waiting for opencode to start")
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return "", ctx.Err()
	}
}

type openCodeSession struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Directory string `json:"directory"`
	Version   int    `json:"version"`
}

func (a *OpenCodeAgent) createSession(ctx context.Context) (*openCodeSession, error) {
	respBody, err := a.doPost(ctx, "/session/", map[string]any{})
	if err != nil {
		return nil, err
	}
	var session openCodeSession
	if err := json.Unmarshal(respBody, &session); err != nil {
		return nil, fmt.Errorf("parse session response: %w", err)
	}
	return &session, nil
}

func (a *OpenCodeAgent) subscribeEvents(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.endpoint+"/event", nil)
	if err != nil {
		logger.Error("opencode event subscribe: %v", err)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == nil {
			logger.Error("opencode event stream error: %v", err)
		}
		return
	}
	defer resp.Body.Close()

	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := resp.Body.Read(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if err == io.EOF {
				logger.Info("opencode event stream closed, reconnecting")
				time.Sleep(time.Second)
				go a.subscribeEvents(ctx)
				return
			}
			logger.Error("opencode event read error: %v", err)
			return
		}

		chunk := string(buf[:n])
		for _, line := range strings.Split(chunk, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			var raw map[string]any
			if err := json.Unmarshal([]byte(data), &raw); err != nil {
				continue
			}

			eventType, _ := raw["type"].(string)
			props, _ := raw["properties"].(map[string]any)

			a.emit(Event{
				Type:           eventType,
				ConversationID: a.sessionID,
				Backend:        "opencode",
				Data:           props,
			})
		}
	}
}

func (a *OpenCodeAgent) doGet(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.endpoint+path, nil)
	if err != nil {
		return nil, err
	}
	return a.httpClient.Do(req)
}

func (a *OpenCodeAgent) doPost(ctx context.Context, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return json.RawMessage(respBody), nil
}
