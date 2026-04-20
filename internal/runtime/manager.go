package runtime

import (
	"context"
	"fmt"
	"sync"

	"cs-cloud/internal/agent"
	agentcs "cs-cloud/internal/agent/cs"
	agentcsc "cs-cloud/internal/agent/csc"
)

type AgentManager struct {
	mu       sync.RWMutex
	agents   map[string]agent.Agent
	drivers  map[string]agent.Driver
	eventBus *EventBus
}

func NewAgentManager(eventBus *EventBus) *AgentManager {
	return &AgentManager{
		agents:   make(map[string]agent.Agent),
		drivers:  make(map[string]agent.Driver),
		eventBus: eventBus,
	}
}

func (m *AgentManager) RegisterDriver(d agent.Driver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.drivers[d.Name()] = d
}

func (m *AgentManager) GetDriver(name string) (agent.Driver, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.drivers[name]
	return d, ok
}

func (m *AgentManager) ResolveDriver(backend string) (agent.Driver, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	d, ok := m.drivers[backend]
	if ok {
		return d, nil
	}

	return nil, fmt.Errorf("no driver for backend: %s", backend)
}

func (m *AgentManager) CreateAgent(ctx context.Context, convID string, cfg agent.AgentConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agents[convID]; exists {
		return fmt.Errorf("agent already exists for conversation: %s", convID)
	}

	d, ok := m.drivers[cfg.Backend]
	if !ok {
		return fmt.Errorf("no driver for backend: %s", cfg.Backend)
	}

	a, err := d.CreateAgent(cfg)
	if err != nil {
		return err
	}

	a.SetEventEmitter(func(event agent.Event) {
		event.ConversationID = convID
		if m.eventBus != nil {
			m.eventBus.Emit(event)
		}
	})

	if err := a.Start(ctx); err != nil {
		return fmt.Errorf("start agent: %w", err)
	}

	m.agents[convID] = a
	return nil
}

func (m *AgentManager) GetAgent(convID string) (agent.Agent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.agents[convID]
	return a, ok
}

func (m *AgentManager) ListAgents() []agent.Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]agent.Agent, 0, len(m.agents))
	for _, a := range m.agents {
		result = append(result, a)
	}
	return result
}

func (m *AgentManager) SendMessage(ctx context.Context, convID string, msg agent.PromptMessage) error {
	a, ok := m.GetAgent(convID)
	if !ok {
		return fmt.Errorf("agent not found: %s", convID)
	}
	return a.SendMessage(ctx, msg)
}

func (m *AgentManager) CancelPrompt(ctx context.Context, convID string) error {
	a, ok := m.GetAgent(convID)
	if !ok {
		return fmt.Errorf("agent not found: %s", convID)
	}
	return a.CancelPrompt(ctx)
}

func (m *AgentManager) ConfirmPermission(ctx context.Context, convID string, callID string, optionID string) error {
	a, ok := m.GetAgent(convID)
	if !ok {
		return fmt.Errorf("agent not found: %s", convID)
	}
	return a.ConfirmPermission(ctx, callID, optionID)
}

func (m *AgentManager) KillAgent(convID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if a, ok := m.agents[convID]; ok {
		_ = a.Kill()
		delete(m.agents, convID)
	}
}

func (m *AgentManager) KillAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, a := range m.agents {
		_ = a.Kill()
		delete(m.agents, id)
	}
}

func (m *AgentManager) AgentPID() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, a := range m.agents {
		if pid := a.PID(); pid > 0 {
			return pid
		}
	}
	return 0
}

func (m *AgentManager) DetectAgents(ctx context.Context) ([]agent.DetectedAgent, error) {
	m.mu.RLock()
	drivers := make([]agent.Driver, 0, len(m.drivers))
	for _, d := range m.drivers {
		drivers = append(drivers, d)
	}
	m.mu.RUnlock()

	var all []agent.DetectedAgent
	for _, d := range drivers {
		agents, err := d.Detect(ctx)
		if err != nil {
			continue
		}
		all = append(all, agents...)
	}
	return all, nil
}

func (m *AgentManager) Endpoint() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, a := range m.agents {
		if e, ok := a.(interface{ Endpoint() string }); ok {
			if ep := e.Endpoint(); ep != "" {
				return ep
			}
		}
	}
	return ""
}

func (m *AgentManager) DefaultBackend() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, a := range m.agents {
		return a.Backend()
	}
	return ""
}

func (m *AgentManager) InitDefaultAgent(ctx context.Context, agentType string, cliPath string, agentEnv map[string]string) error {
	drivers := map[string]agent.Driver{
		"cs":  agentcs.NewDriver(cliPath),
		"csc": agentcsc.NewDriver(cliPath),
	}

	for name, d := range drivers {
		m.RegisterDriver(d)
		_ = name
	}

	if agentType == "" {
		agentType = "cs"
	}

	d, ok := drivers[agentType]
	if !ok {
		return fmt.Errorf("unknown agent type: %s (available: cs, csc)", agentType)
	}

	resolved := cliPath
	if resolved == "" {
		switch agentType {
		case "cs":
			resolved = agentcs.CLIBinary
		case "csc":
			resolved = agentcsc.CLIBinary
		}
	}

	detected, _ := d.Detect(ctx)
	if len(detected) == 0 || !detected[0].Available {
		return fmt.Errorf("agent CLI '%s' not found in PATH, please ensure it is installed correctly", resolved)
	}

	var extra map[string]any
	if v, ok := detected[0].Extra.(map[string]any); ok {
		extra = v
	}
	cfg := agent.AgentConfig{
		ID:         "default",
		Backend:    agentType,
		DriverName: "http",
		WorkingDir: "",
		CustomEnv:  agentEnv,
		Extra:      extra,
	}
	return m.CreateAgent(ctx, "default", cfg)
}

func (m *AgentManager) HealthCheck(ctx context.Context, backend string) (*agent.HealthResult, error) {
	d, err := m.ResolveDriver(backend)
	if err != nil {
		return nil, err
	}
	return d.HealthCheck(ctx, backend)
}
