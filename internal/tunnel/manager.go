package tunnel

import (
	"context"
	"sync"
	"time"

	"cs-cloud/internal/config"
	"cs-cloud/internal/logger"
)

type Manager struct {
	mu        sync.Mutex
	cancel    context.CancelFunc
	localPort int
	connected bool
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) SetCancel(cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancel = cancel
}

func (m *Manager) SetConnected(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = v
}

func (m *Manager) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

func (m *Manager) Reconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		logger.Info("[tunnel-manager] cancelling current tunnel connection for reconnect")
		m.cancel()
		m.cancel = nil
		m.connected = false
	}
}

func RunManagedTunnel(ctx context.Context, localPort int, mgr *Manager, cfg *config.Config) {
	for {
		select {
		case <-ctx.Done():
			logger.Info("[tunnel-manager] tunnel manager stopped")
			return
		default:
		}

		tunnelCtx, cancel := context.WithCancel(ctx)
		mgr.SetCancel(cancel)
		mgr.SetConnected(false)

		logger.Info("[tunnel-manager] starting tunnel connection (port=%d)", localPort)
		err := Connect(tunnelCtx, localPort, cfg)
		if err != nil {
			logger.Warn("[tunnel-manager] tunnel error: %v", err)
		}

		mgr.SetConnected(false)

		select {
		case <-ctx.Done():
			logger.Info("[tunnel-manager] tunnel manager stopped")
			return
		case <-time.After(time.Second):
		}

		logger.Info("[tunnel-manager] tunnel disconnected, reconnecting...")
	}
}
