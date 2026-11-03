// Package api serves the UI and a small JSON RPC on 127.0.0.1, protected by a random token.
package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"paydar/internal/app"
	"paydar/internal/store"
	"paydar/web"
)

type Server struct {
	App   *app.App
	Token string
	// Platform is sent to the UI: "windows", "android", "linux".
	Platform string
	Version  string
	// Native handles connect/disconnect when the platform must do it (Android's VpnService).
	// nil: the app connects directly.
	OnConnect    func() error
	OnDisconnect func()
	OnQuit       func()

	ln   net.Listener
	port int
}

func NewToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Listen binds 127.0.0.1 on port (0 = any free port) and serves in the background.
func (s *Server) Listen(port int) error {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
	}
	s.ln = ln
	s.port = ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: s.handler(), ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(ln)
	return nil
}

func (s *Server) URL() string {
	return "http://127.0.0.1:" + strconv.Itoa(s.port) + "/#t=" + s.Token
}

func (s *Server) Close() {
	if s.ln != nil {
		s.ln.Close()
	}
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(web.Files, ".")
	files := http.FileServer(http.FS(static))
	mux.HandleFunc("/api/", s.rpc)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'")
		files.ServeHTTP(w, r)
	}))
	return s.guardHost(mux)
}

// guardHost rejects other Host headers (DNS rebinding) before anything else.
func (s *Server) guardHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.Host)
		if host != "127.0.0.1" && host != "localhost" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type reply struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Data  any    `json:"data,omitempty"`
}

func (s *Server) rpc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Token")), []byte(s.Token)) != 1 {
		http.Error(w, "bad token", http.StatusUnauthorized)
		return
	}
	var args struct {
		ID       string          `json:"id"`
		IDs      []string        `json:"ids"`
		Text     string          `json:"text"`
		URL      string          `json:"url"`
		Name     string          `json:"name"`
		Settings *store.Settings `json:"settings"`
	}
	json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<20)).Decode(&args)
	a := s.App
	var (
		data any
		err  error
	)
	switch strings.TrimPrefix(r.URL.Path, "/api/") {
	case "hello":
		data = map[string]any{"platform": s.Platform, "version": s.Version, "settings": a.Settings()}
	case "status":
		data = a.Status()
	case "servers":
		data = a.Servers()
	case "subs":
		data = a.Subs()
	case "settings":
		data = a.Settings()
	case "saveSettings":
		if args.Settings != nil {
			a.SetSettings(*args.Settings)
		}
		data = a.Settings()
	case "add":
		data, err = a.Add(args.Text)
	case "addSub":
		data, err = a.AddSub(args.URL, args.Name)
	case "updateSub":
		data, err = a.UpdateSub(args.ID)
	case "updateSubs":
		go a.UpdateAllSubs()
	case "removeSub":
		a.RemoveSub(args.ID)
	case "remove":
		ids := args.IDs
		if args.ID != "" {
			ids = append(ids, args.ID)
		}
		a.Remove(ids)
	case "removeDead":
		data = a.RemoveDead()
	case "link":
		data, err = a.ServerLink(args.ID)
	case "choose":
		err = a.Choose(args.ID)
	case "clearChoice":
		a.ClearChoice()
	case "test":
		go a.TestAll()
	case "connect":
		if s.OnConnect != nil {
			err = s.OnConnect()
		} else {
			go a.Connect()
		}
	case "disconnect":
		if s.OnDisconnect != nil {
			s.OnDisconnect()
		} else {
			go a.Disconnect()
		}
	case "logs":
		data = a.Logs.Lines()
	case "quit":
		if s.OnQuit != nil {
			go s.OnQuit()
		}
	default:
		http.Error(w, "unknown method", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	rep := reply{OK: err == nil, Data: data}
	if err != nil {
		rep.Error = err.Error()
	}
	json.NewEncoder(w).Encode(rep)
}
