package session

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	verrors "github.com/nerveband/vflow/internal/errors"
	vframing "github.com/nerveband/vflow/internal/framing"
	vproject "github.com/nerveband/vflow/internal/project"
)

//go:embed assets/*
var assets embed.FS

type Options struct {
	ProjectPath     string
	Source          string
	Listen          string
	Open            bool
	Wait            bool
	Timeout         time.Duration
	ShutdownToken   string
	CommitEnabled   bool
	OpenBrowserFunc func(string) error
}

type Result struct {
	SessionID            string            `json:"session_id"`
	URL                  string            `json:"url"`
	HealthURL            string            `json:"health_url"`
	StatusURL            string            `json:"status_url"`
	ShutdownURL          string            `json:"shutdown_url"`
	Artifacts            map[string]string `json:"artifacts"`
	Port                 int               `json:"port"`
	PID                  int               `json:"pid"`
	Timeout              string            `json:"timeout"`
	ShutdownTokenPresent bool              `json:"shutdown_token_present"`
	Status               string            `json:"status"`
	CommitEnabled        bool              `json:"commit_enabled"`
}

type Server struct {
	result Result
	http   *http.Server
	ln     net.Listener
	done   chan struct{}
	once   sync.Once
	mu     sync.Mutex
	state  State
	root   string
	source string
	token  string
	commit bool
}

type State struct {
	SessionID        string              `json:"session_id"`
	ProjectID        string              `json:"project_id,omitempty"`
	Status           string              `json:"status"`
	CommitEnabled    bool                `json:"commit_enabled"`
	Artifacts        map[string]string   `json:"artifacts"`
	MediaURL         string              `json:"media_url,omitempty"`
	Presets          vframing.Presets    `json:"presets"`
	SpeakerMap       vframing.SpeakerMap `json:"speaker_map"`
	Policy           vframing.Policy     `json:"policy"`
	SafeZoneWarnings []string            `json:"safe_zone_warnings"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

func Start(ctx context.Context, opts Options) (*Server, Result, *verrors.Error) {
	if opts.ProjectPath == "" {
		opts.ProjectPath = "."
	}
	if opts.Listen == "" {
		opts.Listen = "127.0.0.1:0"
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Minute
	}
	host, _, err := net.SplitHostPort(opts.Listen)
	if err != nil {
		return nil, Result{}, verrors.Validation("CALIBRATE_LISTEN_INVALID", err.Error(), "Use --listen 127.0.0.1:0 or another localhost port", false)
	}
	if host != "127.0.0.1" && host != "localhost" {
		return nil, Result{}, verrors.Validation("CALIBRATE_LISTEN_NOT_LOCALHOST", "calibration sessions bind only to localhost", "Use --listen 127.0.0.1:0", false)
	}
	root, err := filepath.Abs(opts.ProjectPath)
	if err != nil {
		return nil, Result{}, verrors.External("PROJECT_PATH_INVALID", err.Error(), "Check --project", false)
	}
	proj, err := vproject.Load(root)
	if err != nil {
		return nil, Result{}, verrors.External("PROJECT_READ_FAILED", err.Error(), "Run project init --commit first", false)
	}
	source, verr := resolveSource(root, opts.Source)
	if verr != nil {
		return nil, Result{}, verr
	}
	presets, _ := vframing.ReadPresets(root)
	if presets.Version == "" {
		presets = defaultPresets()
	}
	speakerMap, _ := vframing.ReadSpeakerMap(root)
	if speakerMap.Version == "" {
		speakerMap = vframing.SpeakerMap{Version: "vflow-speaker-map/v1", Map: map[string]string{}}
	}
	policy, _ := vframing.ReadPolicy(root)
	if policy.Version == "" {
		policy = vframing.DefaultPolicy()
	}

	ln, err := net.Listen("tcp", opts.Listen)
	if err != nil {
		return nil, Result{}, verrors.External("CALIBRATE_LISTEN_FAILED", err.Error(), "Choose another localhost port", true)
	}
	id := newID()
	port := ln.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	artifacts := artifactPaths(root)
	state := State{
		SessionID:     id,
		ProjectID:     proj.ID,
		Status:        "running",
		CommitEnabled: opts.CommitEnabled,
		Artifacts:     artifacts,
		MediaURL:      "/media/source",
		Presets:       presets,
		SpeakerMap:    speakerMap,
		Policy:        policy,
		UpdatedAt:     time.Now().UTC(),
	}
	s := &Server{ln: ln, done: make(chan struct{}), root: root, source: source, token: opts.ShutdownToken, commit: opts.CommitEnabled, state: state}
	s.result = Result{
		SessionID:            id,
		URL:                  baseURL + "/",
		HealthURL:            baseURL + "/healthz",
		StatusURL:            baseURL + "/api/status",
		ShutdownURL:          baseURL + shutdownPath(opts.ShutdownToken),
		Artifacts:            artifacts,
		Port:                 port,
		PID:                  os.Getpid(),
		Timeout:              opts.Timeout.String(),
		ShutdownTokenPresent: opts.ShutdownToken != "",
		Status:               "running",
		CommitEnabled:        opts.CommitEnabled,
	}
	mux := http.NewServeMux()
	s.routes(mux)
	s.http = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := s.persistStatus(); err != nil {
		_ = ln.Close()
		return nil, Result{}, verrors.External("CALIBRATE_STATUS_WRITE_FAILED", err.Error(), "Check tmp/sessions write permissions", false)
	}
	go func() {
		defer close(s.done)
		if err := s.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.setStatus("failed")
		}
	}()
	if opts.Open {
		open := opts.OpenBrowserFunc
		if open == nil {
			open = openBrowser
		}
		if err := open(s.result.URL); err != nil {
			_ = s.Shutdown(context.Background())
			return nil, Result{}, verrors.External("CALIBRATE_OPEN_FAILED", err.Error(), "Retry with --open=false and open the URL manually", false)
		}
	}
	if opts.Wait {
		go func() {
			<-ctx.Done()
			_ = s.Shutdown(context.Background())
		}()
		select {
		case <-s.done:
		case <-time.After(opts.Timeout):
			s.setStatus("timeout")
			_ = s.Shutdown(context.Background())
		}
	}
	return s, s.result, nil
}

func (s *Server) Result() Result { return s.result }

func (s *Server) Done() <-chan struct{} { return s.done }

func (s *Server) Shutdown(ctx context.Context) error {
	var err error
	s.once.Do(func() {
		s.setStatus("shutdown")
		err = s.http.Shutdown(ctx)
	})
	return err
}

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/static/", s.static)
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/api/status", s.apiState)
	mux.HandleFunc("/api/state", s.apiState)
	mux.HandleFunc("/api/presets", s.apiPresets)
	mux.HandleFunc("/api/speaker-map", s.apiSpeakerMap)
	mux.HandleFunc("/api/policy", s.apiPolicy)
	mux.HandleFunc("/api/commit", s.apiCommit)
	mux.HandleFunc("/api/shutdown", s.apiShutdown)
	mux.HandleFunc("/media/source", s.media)
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	serveAsset(w, r, "assets/index.html")
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	name := path.Clean(strings.TrimPrefix(r.URL.Path, "/static/"))
	if name == "." || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	serveAsset(w, r, "assets/"+name)
}

func serveAsset(w http.ResponseWriter, r *http.Request, name string) {
	raw, err := assets.ReadFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		w.Header().Set("content-type", ct)
	}
	_, _ = w.Write(raw)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeAPI(w, map[string]any{"ok": true, "session_id": s.result.SessionID, "status": s.snapshot().Status})
}

func (s *Server) apiState(w http.ResponseWriter, r *http.Request) {
	writeAPI(w, s.snapshot())
}

func (s *Server) apiPresets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var presets vframing.Presets
	if err := decodeJSON(r.Body, &presets); err != nil {
		writeAPIError(w, http.StatusBadRequest, "CALIBRATE_PRESETS_INVALID_JSON", err.Error())
		return
	}
	if err := presets.Validate(); err != nil {
		writeAPIError(w, http.StatusBadRequest, "CALIBRATE_PRESETS_INVALID", err.Error())
		return
	}
	s.mu.Lock()
	s.state.Presets = presets
	s.state.SafeZoneWarnings = safeZoneWarnings(presets)
	s.state.UpdatedAt = time.Now().UTC()
	s.mu.Unlock()
	_ = s.persistStatus()
	writeAPI(w, map[string]any{"status": "accepted", "state": s.snapshot()})
}

func (s *Server) apiSpeakerMap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var speakerMap vframing.SpeakerMap
	if err := decodeJSON(r.Body, &speakerMap); err != nil {
		writeAPIError(w, http.StatusBadRequest, "CALIBRATE_SPEAKER_MAP_INVALID_JSON", err.Error())
		return
	}
	if speakerMap.Version == "" {
		speakerMap.Version = "vflow-speaker-map/v1"
	}
	if speakerMap.Map == nil {
		speakerMap.Map = map[string]string{}
	}
	presetIDs := s.snapshot().Presets.IDSet()
	if err := speakerMap.Validate(presetIDs); err != nil {
		writeAPIError(w, http.StatusBadRequest, "CALIBRATE_SPEAKER_MAP_INVALID", err.Error())
		return
	}
	s.mu.Lock()
	s.state.SpeakerMap = speakerMap
	s.state.UpdatedAt = time.Now().UTC()
	s.mu.Unlock()
	_ = s.persistStatus()
	writeAPI(w, map[string]any{"status": "accepted", "state": s.snapshot()})
}

func (s *Server) apiPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var policy vframing.Policy
	if err := decodeJSON(r.Body, &policy); err != nil {
		writeAPIError(w, http.StatusBadRequest, "CALIBRATE_POLICY_INVALID_JSON", err.Error())
		return
	}
	if policy.Version == "" {
		policy.Version = "vflow-framing-policy/v1"
	}
	s.mu.Lock()
	s.state.Policy = policy
	s.state.UpdatedAt = time.Now().UTC()
	s.mu.Unlock()
	_ = s.persistStatus()
	writeAPI(w, map[string]any{"status": "accepted", "state": s.snapshot()})
}

func (s *Server) apiCommit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.commit || !commitIntent(r) {
		writeAPIError(w, http.StatusForbidden, "SAFETY_COMMIT_REQUIRED", "writes require session --commit and API commit intent")
		return
	}
	state := s.snapshot()
	if err := vframing.WritePresets(s.root, state.Presets); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "FRAMING_PRESET_WRITE_FAILED", err.Error())
		return
	}
	if err := writeJSONFile(filepath.Join(s.root, "calibration", "speaker-map.json"), state.SpeakerMap); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "SPEAKER_MAP_WRITE_FAILED", err.Error())
		return
	}
	if err := writeJSONFile(filepath.Join(s.root, "policy", "framing-policy.json"), state.Policy); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "FRAMING_POLICY_WRITE_FAILED", err.Error())
		return
	}
	s.setStatus("committed")
	writeAPI(w, map[string]any{"status": "written", "artifacts": state.Artifacts})
}

func (s *Server) apiShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.token != "" && r.URL.Query().Get("token") != s.token && r.Header.Get("X-Vflow-Shutdown-Token") != s.token {
		writeAPIError(w, http.StatusForbidden, "CALIBRATE_SHUTDOWN_TOKEN_INVALID", "valid shutdown token required")
		return
	}
	writeAPI(w, map[string]any{"status": "shutting_down", "session_id": s.result.SessionID})
	go func() {
		time.Sleep(25 * time.Millisecond)
		_ = s.Shutdown(context.Background())
	}()
}

func (s *Server) media(w http.ResponseWriter, r *http.Request) {
	if s.source == "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, s.source)
}

func (s *Server) snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.state
	state.Artifacts = cloneMap(s.state.Artifacts)
	state.SafeZoneWarnings = append([]string(nil), s.state.SafeZoneWarnings...)
	return state
}

func (s *Server) setStatus(status string) {
	s.mu.Lock()
	s.state.Status = status
	s.state.UpdatedAt = time.Now().UTC()
	s.mu.Unlock()
	_ = s.persistStatus()
}

func (s *Server) persistStatus() error {
	statusPath := filepath.Join(s.root, "tmp", "sessions", s.result.SessionID+".json")
	return writeJSONFile(statusPath, s.snapshot())
}

func resolveSource(root, source string) (string, *verrors.Error) {
	if source == "" {
		return "", nil
	}
	clean := filepath.Clean(source)
	if filepath.IsAbs(clean) {
		if _, err := os.Stat(clean); err != nil {
			return "", verrors.External("CALIBRATE_SOURCE_READ_FAILED", err.Error(), "Check --source exists", false)
		}
		return clean, nil
	}
	full, err := filepath.Abs(filepath.Join(root, clean))
	if err != nil {
		return "", verrors.External("CALIBRATE_SOURCE_INVALID", err.Error(), "Check --source", false)
	}
	rel, err := filepath.Rel(root, full)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "."+string(filepath.Separator) {
		return "", verrors.Validation("CALIBRATE_SOURCE_OUTSIDE_PROJECT", "--source must stay under the project root unless absolute", "Use an absolute proxy/external path or a project-relative media path", false)
	}
	if _, err := os.Stat(full); err != nil {
		return "", verrors.External("CALIBRATE_SOURCE_READ_FAILED", err.Error(), "Check --source exists under the project", false)
	}
	return full, nil
}

func artifactPaths(root string) map[string]string {
	rel := map[string]string{
		"framing_presets": "calibration/framing-presets.json",
		"speaker_map":     "calibration/speaker-map.json",
		"framing_policy":  "policy/framing-policy.json",
		"framing_lane":    "decisions/framing-lane.json",
		"review_queue":    "review/review-queue.json",
		"session_status":  "tmp/sessions",
	}
	out := map[string]string{}
	for key, p := range rel {
		out[key] = filepath.ToSlash(filepath.Join(root, p))
	}
	return out
}

func defaultPresets() vframing.Presets {
	return vframing.Presets{
		Version:      "vflow-framing-presets/v1",
		SourceWidth:  3840,
		SourceHeight: 2160,
		TargetAspect: "16:9",
		Presets: []vframing.Preset{{
			ID: "wide", Label: "Wide", Type: "wide", CropPX: vframing.Rect{X: 0, Y: 0, W: 3840, H: 2160},
		}},
	}
}

func safeZoneWarnings(presets vframing.Presets) []string {
	warnings := []string{}
	for _, preset := range presets.Presets {
		if preset.CropPX.W < presets.SourceWidth/4 || preset.CropPX.H < presets.SourceHeight/4 {
			warnings = append(warnings, fmt.Sprintf("%s exceeds 4x zoom review threshold", preset.ID))
		}
	}
	return warnings
}

func commitIntent(r *http.Request) bool {
	if r.URL.Query().Get("commit") == "true" {
		return true
	}
	var body struct {
		Commit bool `json:"commit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.Commit {
		return true
	}
	return false
}

func decodeJSON(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeAPI(w http.ResponseWriter, value any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":             false,
		"schema_version": "vflow-error/v1",
		"error":          map[string]any{"code": code, "message": message, "retryable": false, "exit_code": 4},
	})
}

func writeJSONFile(file string, value any) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(raw, '\n'), 0o644)
}

func cloneMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func shutdownPath(token string) string {
	if token == "" {
		return "/api/shutdown"
	}
	return "/api/shutdown?token=" + token
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	return "sess_" + hex.EncodeToString(b[:])
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
