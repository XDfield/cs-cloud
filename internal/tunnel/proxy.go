package tunnel

import (
	"fmt"
	"io"
	"net"
	"strings"

	"cs-cloud/internal/logger"
)

func handleStream(stream net.Conn, localPort int) {
	defer stream.Close()

	buf := make([]byte, 64*1024)
	var header []byte

	for {
		n, err := stream.Read(buf)
		if err != nil {
			return
		}
		header = append(header, buf[:n]...)

		idx := findDoubleCRLF(header)
		if idx < 0 {
			continue
		}

		headerStr := string(header[:idx])
		body := header[idx+4:]
		header = nil

		lines := strings.Split(headerStr, "\r\n")
		if len(lines) == 0 {
			return
		}

		parts := strings.SplitN(lines[0], " ", 3)
		if len(parts) < 2 {
			return
		}
		method := parts[0]
		path := parts[1]

		headers := make(map[string]string)
		for _, line := range lines[1:] {
			colonIdx := strings.Index(line, ": ")
			if colonIdx < 0 {
				continue
			}
			headers[strings.ToLower(line[:colonIdx])] = line[colonIdx+2:]
		}

		contentLength := -1
		if cl, ok := headers["content-length"]; ok {
			fmt.Sscanf(cl, "%d", &contentLength)
		}

		logger.Debug("[tunnel] %s %s content-length=%d body-so-far=%d", method, path, contentLength, len(body))

		if contentLength >= 0 {
			for len(body) < contentLength {
				n, err := stream.Read(buf)
				if err != nil {
					return
				}
				body = append(body, buf[:n]...)
			}
			if len(body) > contentLength {
				body = body[:contentLength]
			}
		}

		isWS := strings.ToLower(headers["upgrade"]) == "websocket"
		if isWS {
			proxyWebSocket(stream, method, path, headers, body, localPort)
		} else {
			proxyHTTP(stream, method, path, headers, body, localPort)
		}
		return
	}
}

func proxyHTTP(stream net.Conn, method, path string, headers map[string]string, body []byte, localPort int) {
	localConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return
	}
	defer localConn.Close()

	var req strings.Builder
	req.WriteString(fmt.Sprintf("%s %s HTTP/1.1\r\n", method, path))
	req.WriteString(fmt.Sprintf("Host: 127.0.0.1:%d\r\n", localPort))
	for k, v := range headers {
		switch k {
		case "host", "connection", "transfer-encoding", "content-length":
			continue
		}
		req.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	if len(body) > 0 {
		req.WriteString(fmt.Sprintf("content-length: %d\r\n", len(body)))
	}
	req.WriteString("\r\n")

	localConn.Write([]byte(req.String()))
	if len(body) > 0 {
		localConn.Write(body)
	}

	go func() {
		io.Copy(stream, localConn)
		stream.Close()
	}()
	io.Copy(localConn, stream)
}

func proxyWebSocket(stream net.Conn, method, path string, headers map[string]string, body []byte, localPort int) {
	localConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return
	}
	defer localConn.Close()

	var req strings.Builder
	req.WriteString(fmt.Sprintf("%s %s HTTP/1.1\r\n", method, path))
	req.WriteString(fmt.Sprintf("Host: 127.0.0.1:%d\r\n", localPort))
	for k, v := range headers {
		if k == "host" {
			continue
		}
		req.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	req.WriteString("\r\n")

	localConn.Write([]byte(req.String()))
	if len(body) > 0 {
		localConn.Write(body)
	}

	go func() {
		io.Copy(stream, localConn)
		stream.Close()
	}()
	io.Copy(localConn, stream)
}

func findDoubleCRLF(data []byte) int {
	for i := 0; i <= len(data)-4; i++ {
		if data[i] == '\r' && data[i+1] == '\n' && data[i+2] == '\r' && data[i+3] == '\n' {
			return i
		}
	}
	return -1
}
