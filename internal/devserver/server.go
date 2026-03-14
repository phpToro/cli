package devserver

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/phpToro/cli/internal/ui"
)

const (
	DefaultPort  = 8942
	PollInterval = 500 * time.Millisecond
)

// Server provides hot reload via file watching and WebSocket.
type Server struct {
	projectRoot string
	port        int
	clients     map[net.Conn]bool
	mu          sync.Mutex
	stop        chan struct{}
}

// New creates a new dev server.
func New(projectRoot string, port int) *Server {
	if port == 0 {
		port = DefaultPort
	}
	return &Server{
		projectRoot: projectRoot,
		port:        port,
		clients:     make(map[net.Conn]bool),
		stop:        make(chan struct{}),
	}
}

// LocalIP returns the machine's local network IP (for physical device access).
func GetLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "localhost"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}
			return ip.String()
		}
	}
	return "localhost"
}

// Port returns the server's port.
func (s *Server) Port() int {
	return s.port
}

// Start begins file watching and WebSocket server.
func (s *Server) Start() error {
	// Start WebSocket server
	go s.serveWebSocket()

	// Start file watcher
	go s.watchFiles()

	return nil
}

// Stop shuts down the server.
func (s *Server) Stop() {
	close(s.stop)
	s.mu.Lock()
	for conn := range s.clients {
		conn.Close()
	}
	s.mu.Unlock()
}

// Wait blocks until the server is stopped (e.g. via signal).
func (s *Server) Wait() {
	<-s.stop
}

func (s *Server) serveWebSocket() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgradeWebSocket(w, r)
		if err != nil {
			return
		}
		s.mu.Lock()
		s.clients[conn] = true
		s.mu.Unlock()

		// Read loop — decode WebSocket frames and handle log messages
		s.readLoop(conn)

		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
		conn.Close()
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	go func() {
		<-s.stop
		server.Close()
	}()

	server.ListenAndServe()
}

func (s *Server) watchFiles() {
	// Build initial state of file mtimes
	state := s.scanFiles()

	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			newState := s.scanFiles()
			for path, mtime := range newState {
				oldMtime, exists := state[path]
				if !exists || mtime.After(oldMtime) {
					rel, _ := filepath.Rel(s.projectRoot, path)
					ext := filepath.Ext(path)

					if ext == ".json" && filepath.Base(path) == "phptoro.json" {
						ui.Info("Config changed — reloading app")
						s.broadcast(`{"type":"reload_config"}`)
					} else {
						ui.Dim(fmt.Sprintf("  ↻ %s", rel))
						// Include file content (base64) so physical devices can sync
						content, err := os.ReadFile(path)
						if err == nil {
							encoded := base64.StdEncoding.EncodeToString(content)
							s.broadcast(fmt.Sprintf(`{"type":"reload","file":"%s","content":"%s"}`, rel, encoded))
						} else {
							s.broadcast(fmt.Sprintf(`{"type":"reload","file":"%s"}`, rel))
						}
					}
				}
			}
			state = newState
		}
	}
}

func (s *Server) scanFiles() map[string]time.Time {
	files := make(map[string]time.Time)
	watchExts := map[string]bool{
		".php": true, ".json": true, ".toro": true,
	}

	filepath.Walk(s.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs, vendor, ios/, node_modules
		base := filepath.Base(path)
		if info.IsDir() {
			if strings.HasPrefix(base, ".") || base == "vendor" || base == "ios" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if watchExts[filepath.Ext(path)] {
			files[path] = info.ModTime()
		}
		return nil
	})
	return files
}

func (s *Server) broadcast(message string) {
	frame := encodeWSFrame([]byte(message))
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.clients {
		conn.Write(frame)
	}
}

// --- Minimal WebSocket implementation ---

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "not a websocket request", http.StatusBadRequest)
		return nil, fmt.Errorf("missing Sec-WebSocket-Key")
	}

	// Compute accept key
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Hijack the connection
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return nil, fmt.Errorf("hijack not supported")
	}

	conn, buf, err := hj.Hijack()
	if err != nil {
		return nil, err
	}

	// Send upgrade response
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"
	buf.WriteString(response)
	buf.Flush()

	return conn, nil
}

var (
	logJS    = color.New(color.FgMagenta)
	logJSErr = color.New(color.FgRed, color.Bold)
	logJSWrn = color.New(color.FgYellow)

	devLogPhp    = color.New(color.FgYellow)
	devLogBridge = color.New(color.FgCyan)
	devLogKernel = color.New(color.FgGreen)
	devLogScreen = color.New(color.FgBlue)
	devLogErr    = color.New(color.FgRed, color.Bold)
	devLogTap    = color.New(color.FgHiCyan)
	devLogDef    = color.New(color.FgHiBlack)
)

func formatDevLogLine(msg string, level string) {
	formatted := "  " + msg
	if level == "error" || strings.Contains(msg, "ERROR") {
		devLogErr.Fprintln(os.Stderr, formatted)
	} else if strings.Contains(msg, "[phpToro.PHP]") {
		devLogPhp.Fprintln(os.Stderr, formatted)
	} else if strings.Contains(msg, "[phpToro.Kernel]") {
		devLogKernel.Fprintln(os.Stderr, formatted)
	} else if strings.Contains(msg, "[phpToro.Screen]") {
		devLogScreen.Fprintln(os.Stderr, formatted)
	} else if strings.Contains(msg, "[phpToro.Bridge]") || strings.Contains(msg, "[phpToro.PluginHost]") {
		devLogBridge.Fprintln(os.Stderr, formatted)
	} else if strings.Contains(msg, "[phpToro.Tap]") {
		devLogTap.Fprintln(os.Stderr, formatted)
	} else {
		devLogDef.Fprintln(os.Stderr, formatted)
	}
}

func (s *Server) readLoop(conn net.Conn) {
	buf := make([]byte, 64*1024)
	var carry []byte // leftover bytes from previous read

	for {
		n, err := conn.Read(buf)
		if err != nil {
			break
		}

		data := append(carry, buf[:n]...)
		carry = nil

		for len(data) > 0 {
			payload, consumed, err := decodeWSFrame(data)
			if err != nil {
				// Incomplete frame — save for next read
				carry = data
				break
			}
			data = data[consumed:]
			if payload != nil {
				s.handleClientMessage(payload)
			}
		}
	}
}

type wsLogMessage struct {
	Type    string `json:"type"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

func (s *Server) handleClientMessage(payload []byte) {
	var msg wsLogMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return
	}

	if msg.Type == "log" {
		message := msg.Message
		// If message already has [phpToro...] prefix (from native dbg.log), use formatLogLine
		if strings.Contains(message, "[phpToro") {
			formatDevLogLine(message, msg.Level)
		} else {
			formatted := fmt.Sprintf("  [phpToro.JS] [%s] %s", strings.ToUpper(msg.Level), message)
			switch msg.Level {
			case "error":
				logJSErr.Fprintln(os.Stderr, formatted)
			case "warn":
				logJSWrn.Fprintln(os.Stderr, formatted)
			default:
				logJS.Fprintln(os.Stderr, formatted)
			}
		}
	}
}

// decodeWSFrame decodes a single WebSocket frame from data.
// Returns the unmasked payload, number of bytes consumed, and any error.
// Returns nil payload for close/ping/pong control frames.
func decodeWSFrame(data []byte) (payload []byte, consumed int, err error) {
	if len(data) < 2 {
		return nil, 0, fmt.Errorf("incomplete")
	}

	opcode := data[0] & 0x0F
	masked := data[1]&0x80 != 0
	length := int(data[1] & 0x7F)
	offset := 2

	if length == 126 {
		if len(data) < 4 {
			return nil, 0, fmt.Errorf("incomplete")
		}
		length = int(binary.BigEndian.Uint16(data[2:4]))
		offset = 4
	} else if length == 127 {
		if len(data) < 10 {
			return nil, 0, fmt.Errorf("incomplete")
		}
		length = int(binary.BigEndian.Uint64(data[2:10]))
		offset = 10
	}

	var maskKey []byte
	if masked {
		if len(data) < offset+4 {
			return nil, 0, fmt.Errorf("incomplete")
		}
		maskKey = data[offset : offset+4]
		offset += 4
	}

	if len(data) < offset+length {
		return nil, 0, fmt.Errorf("incomplete")
	}

	consumed = offset + length

	// Close frame
	if opcode == 0x8 {
		return nil, consumed, nil
	}

	// Ping/pong — ignore
	if opcode == 0x9 || opcode == 0xA {
		return nil, consumed, nil
	}

	payload = make([]byte, length)
	copy(payload, data[offset:offset+length])

	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	return payload, consumed, nil
}

func encodeWSFrame(payload []byte) []byte {
	length := len(payload)
	var frame []byte

	// FIN bit + text opcode
	frame = append(frame, 0x81)

	if length < 126 {
		frame = append(frame, byte(length))
	} else if length < 65536 {
		frame = append(frame, 126)
		lenBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(lenBytes, uint16(length))
		frame = append(frame, lenBytes...)
	} else {
		frame = append(frame, 127)
		lenBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(lenBytes, uint64(length))
		frame = append(frame, lenBytes...)
	}

	frame = append(frame, payload...)
	return frame
}
