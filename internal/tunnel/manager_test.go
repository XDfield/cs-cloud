package tunnel

import (
	"context"
	"testing"
	"time"
)

func TestManagerSetConnected(t *testing.T) {
	m := NewManager()

	if m.IsConnected() {
		t.Error("should start disconnected")
	}

	m.SetConnected(true)
	if !m.IsConnected() {
		t.Error("should be connected")
	}

	m.SetConnected(false)
	if m.IsConnected() {
		t.Error("should be disconnected")
	}
}

func TestManagerReconnectCancelsContext(t *testing.T) {
	m := NewManager()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tunnelCtx, tunnelCancel := context.WithCancel(ctx)
	m.SetCancel(tunnelCancel)
	m.SetConnected(true)

	m.Reconnect()

	if m.IsConnected() {
		t.Error("should be disconnected after reconnect")
	}

	select {
	case <-tunnelCtx.Done():
	default:
		t.Error("tunnel context should be cancelled after reconnect")
	}
}

func TestManagerReconnectWithoutCancel(t *testing.T) {
	m := NewManager()

	m.Reconnect()

	if m.IsConnected() {
		t.Error("should remain disconnected")
	}
}

func TestManagerSetCancelOverwrite(t *testing.T) {
	m := NewManager()

	ctx := context.Background()
	_, cancel1 := context.WithCancel(ctx)
	ctx2, cancel2 := context.WithCancel(ctx)

	_ = cancel1

	m.SetCancel(cancel1)
	m.SetCancel(cancel2)

	m.Reconnect()

	select {
	case <-ctx2.Done():
	default:
		t.Error("second cancel should have been called")
	}
}

func TestRunManagedTunnelContextCancel(t *testing.T) {
	m := NewManager()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		RunManagedTunnel(ctx, 0, m, nil)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RunManagedTunnel should exit when context cancelled")
	}
}
