package terminal

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"sync"
	"time"
)

type Session struct {
	ID        string
	Pid       int
	Cols      uint16
	Rows      uint16
	Cwd       string
	CreatedAt time.Time
	LastInput time.Time
	cancel    context.CancelFunc

	subscribers map[string]chan []byte
	subMu       sync.RWMutex

	ptmx io.ReadWriteCloser
}

func (s *Session) Write(data []byte) error {
	s.LastInput = time.Now()
	_, err := s.ptmx.Write(data)
	return err
}

func (s *Session) Resize(rows, cols uint16) error {
	s.Rows = rows
	s.Cols = cols
	return s.resizePty(rows, cols)
}

func (s *Session) Close() {
	s.cancel()
	killProcessTree(s.Pid)
	_ = s.ptmx.Close()

	s.subMu.Lock()
	for id, ch := range s.subscribers {
		close(ch)
		delete(s.subscribers, id)
	}
	s.subMu.Unlock()
}

func (s *Session) Subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, 256)
	id := generateSubID()

	s.subMu.Lock()
	s.subscribers[id] = ch
	s.subMu.Unlock()

	unsub := func() {
		s.subMu.Lock()
		defer s.subMu.Unlock()
		if existing, ok := s.subscribers[id]; ok {
			close(existing)
			delete(s.subscribers, id)
		}
	}

	return ch, unsub
}

func (s *Session) SubscriberCount() int {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	return len(s.subscribers)
}

func (s *Session) readOutput(ctx context.Context) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := s.ptmx.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			s.broadcast(data)
		}
		if err != nil {
			s.broadcastExit()
			return
		}
	}
}

func (s *Session) broadcast(data []byte) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	for _, ch := range s.subscribers {
		select {
		case ch <- data:
		default:
		}
	}
}

func (s *Session) broadcastExit() {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	for _, ch := range s.subscribers {
		select {
		case ch <- nil:
		default:
		}
	}
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("t_%x", b)
}

func generateSubID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
