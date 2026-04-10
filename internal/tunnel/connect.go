package tunnel

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"cs-cloud/internal/device"
	"cs-cloud/internal/logger"

	"github.com/hashicorp/yamux"
	"nhooyr.io/websocket"
)

const (
	initialDelay     = 1 * time.Second
	maxDelay         = 60 * time.Second
	wsConnectTimeout = 15 * time.Second
)

func Connect(ctx context.Context, localPort int) error {
	attempt := 0
	for {
		dev, err := device.LoadDevice()
		if err != nil || dev == nil {
			return fmt.Errorf("device not registered, cannot connect tunnel")
		}

		gatewayURL, err := device.AssignGateway(ctx, dev)
		if err != nil {
			logger.Warn("[tunnel] gateway-assign failed: %v, retrying...", err)
			delay := backoff(attempt)
			attempt++
			time.Sleep(delay)
			continue
		}

		err = runSession(ctx, gatewayURL, dev.DeviceID, dev.DeviceToken, localPort)
		if err != nil {
			logger.Warn("[tunnel] session error: %v", err)
		}

		logger.Info("[tunnel] session ended, reconnecting...")
		attempt = 0
		time.Sleep(initialDelay)
	}
}

func runSession(ctx context.Context, gatewayURL, deviceID, deviceToken string, localPort int) error {
	wsURL := strings.Replace(gatewayURL, "http", "ws", 1)
	wsURL = fmt.Sprintf("%s/device/%s/tunnel?token=%s", wsURL, deviceID, url.QueryEscape(deviceToken))

	logger.Info("[tunnel] connecting to %s", redactToken(wsURL))

	connectCtx, cancel := context.WithTimeout(ctx, wsConnectTimeout)
	defer cancel()

	conn, _, err := websocket.Dial(connectCtx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("ws connect failed: %w", err)
	}

	logger.Info("[tunnel] connected, device_id=%s", deviceID)

	wsNetConn := &wsNetConn{Conn: conn}
	defer wsNetConn.Close()

	yamuxCfg := yamux.DefaultConfig()
	yamuxCfg.EnableKeepAlive = false
	yamuxCfg.ConnectionWriteTimeout = 60 * time.Second

	session, err := yamux.Client(wsNetConn, yamuxCfg)
	if err != nil {
		return fmt.Errorf("yamux client init failed: %w", err)
	}
	defer session.Close()

	for {
		stream, err := session.Accept()
		if err != nil {
			return fmt.Errorf("yamux accept failed: %w", err)
		}
		go handleStream(stream, localPort)
	}
}

func backoff(attempt int) time.Duration {
	d := initialDelay * time.Duration(1<<uint(attempt))
	if d > maxDelay {
		d = maxDelay
	}
	return d
}

func redactToken(s string) string {
	idx := strings.Index(s, "token=")
	if idx < 0 {
		return s
	}
	end := strings.Index(s[idx:], "&")
	if end < 0 {
		return s[:idx+6] + "***"
	}
	return s[:idx+6] + "***" + s[idx+end:]
}

type wsNetConn struct {
	*websocket.Conn
	reader io.Reader
	mu     sync.Mutex
}

func (c *wsNetConn) Read(b []byte) (int, error) {
	for {
		if c.reader != nil {
			n, err := c.reader.Read(b)
			if err == io.EOF {
				c.reader = nil
				continue
			}
			return n, err
		}
		_, msg, err := c.Conn.Read(context.Background())
		if err != nil {
			return 0, err
		}
		c.reader = bytes.NewReader(msg)
	}
}

func (c *wsNetConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.Conn.Write(context.Background(), websocket.MessageBinary, b)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *wsNetConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *wsNetConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *wsNetConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *wsNetConn) LocalAddr() net.Addr  { return &net.TCPAddr{} }
func (c *wsNetConn) RemoteAddr() net.Addr { return &net.TCPAddr{} }

func (c *wsNetConn) Close() error {
	return c.Conn.Close(websocket.StatusNormalClosure, "closing")
}
