package server

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard-bellwether/internal/store"
)

type Server struct {
	db  *store.DB
	mux *http.ServeMux
}

func New(db *store.DB) *Server {
	s := &Server{db: db, mux: http.NewServeMux()}

	// Monitors
	s.mux.HandleFunc("GET /api/monitors", s.listMonitors)
	s.mux.HandleFunc("POST /api/monitors", s.createMonitor)
	s.mux.HandleFunc("GET /api/monitors/{id}", s.getMonitor)
	s.mux.HandleFunc("PUT /api/monitors/{id}", s.updateMonitor)
	s.mux.HandleFunc("DELETE /api/monitors/{id}", s.deleteMonitor)
	s.mux.HandleFunc("POST /api/monitors/{id}/check", s.triggerCheck)
	s.mux.HandleFunc("POST /api/monitors/{id}/pause", s.pauseMonitor)
	s.mux.HandleFunc("POST /api/monitors/{id}/resume", s.resumeMonitor)

	// Checks
	s.mux.HandleFunc("GET /api/monitors/{id}/checks", s.listChecks)

	// Incidents
	s.mux.HandleFunc("GET /api/incidents", s.listIncidents)
	s.mux.HandleFunc("GET /api/monitors/{id}/incidents", s.listMonitorIncidents)

	// Alert Rules
	s.mux.HandleFunc("GET /api/monitors/{id}/alerts", s.listAlerts)
	s.mux.HandleFunc("POST /api/monitors/{id}/alerts", s.createAlert)
	s.mux.HandleFunc("DELETE /api/alerts/{id}", s.deleteAlert)

	// Meta
	s.mux.HandleFunc("GET /api/stats", s.stats)
	s.mux.HandleFunc("GET /api/health", s.health)

	// Dashboard
	s.mux.HandleFunc("GET /ui", s.dashboard)
	s.mux.HandleFunc("GET /ui/", s.dashboard)
	s.mux.HandleFunc("GET /", s.root)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (s *Server) root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/ui", http.StatusFound)
}

// ── Monitors ──

func (s *Server) listMonitors(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"monitors": orEmpty(s.db.ListMonitors())})
}

func (s *Server) createMonitor(w http.ResponseWriter, r *http.Request) {
	var m store.Monitor
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeErr(w, 400, "invalid json")
		return
	}
	if m.Name == "" || m.URL == "" {
		writeErr(w, 400, "name and url required")
		return
	}
	if err := s.db.CreateMonitor(&m); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, s.db.GetMonitor(m.ID))
}

func (s *Server) getMonitor(w http.ResponseWriter, r *http.Request) {
	m := s.db.GetMonitor(r.PathValue("id"))
	if m == nil {
		writeErr(w, 404, "not found")
		return
	}
	writeJSON(w, 200, m)
}

func (s *Server) updateMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing := s.db.GetMonitor(id)
	if existing == nil {
		writeErr(w, 404, "not found")
		return
	}
	var m store.Monitor
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeErr(w, 400, "invalid json")
		return
	}
	if m.Name == "" {
		m.Name = existing.Name
	}
	if m.URL == "" {
		m.URL = existing.URL
	}
	if m.Type == "" {
		m.Type = existing.Type
	}
	if m.IntervalSec <= 0 {
		m.IntervalSec = existing.IntervalSec
	}
	if m.TimeoutSec <= 0 {
		m.TimeoutSec = existing.TimeoutSec
	}
	if m.ExpectedStatus <= 0 {
		m.ExpectedStatus = existing.ExpectedStatus
	}
	if m.Method == "" {
		m.Method = existing.Method
	}
	if m.Headers == nil {
		m.Headers = existing.Headers
	}
	if err := s.db.UpdateMonitor(id, &m); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, s.db.GetMonitor(id))
}

func (s *Server) deleteMonitor(w http.ResponseWriter, r *http.Request) {
	if err := s.db.DeleteMonitor(r.PathValue("id")); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"deleted": "ok"})
}

func (s *Server) pauseMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m := s.db.GetMonitor(id)
	if m == nil {
		writeErr(w, 404, "not found")
		return
	}
	m.Paused = true
	s.db.UpdateMonitor(id, m)
	writeJSON(w, 200, s.db.GetMonitor(id))
}

func (s *Server) resumeMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m := s.db.GetMonitor(id)
	if m == nil {
		writeErr(w, 404, "not found")
		return
	}
	m.Paused = false
	s.db.UpdateMonitor(id, m)
	writeJSON(w, 200, s.db.GetMonitor(id))
}

func (s *Server) triggerCheck(w http.ResponseWriter, r *http.Request) {
	m := s.db.GetMonitor(r.PathValue("id"))
	if m == nil {
		writeErr(w, 404, "not found")
		return
	}
	c := RunCheck(m)
	s.db.RecordCheck(&c)
	s.maybeAlert(m, &c)
	writeJSON(w, 200, c)
}

// ── Checks ──

func (s *Server) listChecks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, 200, map[string]any{"checks": orEmpty(s.db.ListChecks(id, limit))})
}

// ── Incidents ──

func (s *Server) listIncidents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, 200, map[string]any{"incidents": orEmpty(s.db.ListIncidents("", limit))})
}

func (s *Server) listMonitorIncidents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, 200, map[string]any{"incidents": orEmpty(s.db.ListIncidents(id, limit))})
}

// ── Alert Rules ──

func (s *Server) listAlerts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"alerts": orEmpty(s.db.ListAlertRules(r.PathValue("id")))})
}

func (s *Server) createAlert(w http.ResponseWriter, r *http.Request) {
	monitorID := r.PathValue("id")
	if s.db.GetMonitor(monitorID) == nil {
		writeErr(w, 404, "monitor not found")
		return
	}
	var a store.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeErr(w, 400, "invalid json")
		return
	}
	if a.Endpoint == "" {
		writeErr(w, 400, "endpoint required")
		return
	}
	a.MonitorID = monitorID
	a.Enabled = true
	if err := s.db.CreateAlertRule(&a); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, a)
}

func (s *Server) deleteAlert(w http.ResponseWriter, r *http.Request) {
	if err := s.db.DeleteAlertRule(r.PathValue("id")); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"deleted": "ok"})
}

// ── Meta ──

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.db.Stats())
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	st := s.db.Stats()
	writeJSON(w, 200, map[string]any{"status": "ok", "service": "bellwether", "monitors": st.Monitors, "up": st.Up, "down": st.Down})
}

// ── Background Checker ──

func (s *Server) StartChecker() {
	go func() {
		for {
			monitors := s.db.ActiveMonitors()
			for _, m := range monitors {
				lastCheck, ok := s.db.LastCheckTime(m.ID)
				if ok && time.Since(lastCheck) < time.Duration(m.IntervalSec)*time.Second {
					continue
				}
				mc := m // copy
				c := RunCheck(&mc)
				s.db.RecordCheck(&c)
				s.maybeAlert(&mc, &c)
			}
			time.Sleep(10 * time.Second)
		}
	}()
}

func RunCheck(m *store.Monitor) store.Check {
	c := store.Check{MonitorID: m.ID}
	start := time.Now()

	switch m.Type {
	case "tcp":
		conn, err := net.DialTimeout("tcp", m.URL, time.Duration(m.TimeoutSec)*time.Second)
		c.RespTimeMs = int(time.Since(start).Milliseconds())
		if err != nil {
			c.Status = "down"
			c.ErrorMsg = err.Error()
		} else {
			conn.Close()
			c.Status = "up"
		}

	case "dns":
		parts := strings.SplitN(m.URL, ":", 2)
		host := parts[0]
		_, err := net.LookupHost(host)
		c.RespTimeMs = int(time.Since(start).Milliseconds())
		if err != nil {
			c.Status = "down"
			c.ErrorMsg = err.Error()
		} else {
			c.Status = "up"
		}

	default: // http
		client := &http.Client{
			Timeout: time.Duration(m.TimeoutSec) * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}
		method := m.Method
		if method == "" {
			method = "GET"
		}
		req, err := http.NewRequest(method, m.URL, nil)
		if err != nil {
			c.Status = "down"
			c.ErrorMsg = err.Error()
			c.RespTimeMs = int(time.Since(start).Milliseconds())
			return c
		}
		req.Header.Set("User-Agent", "Bellwether/1.0")
		for k, v := range m.Headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		c.RespTimeMs = int(time.Since(start).Milliseconds())
		if err != nil {
			c.Status = "down"
			c.ErrorMsg = err.Error()
			return c
		}
		resp.Body.Close()
		c.StatusCode = resp.StatusCode
		if m.ExpectedStatus > 0 && resp.StatusCode != m.ExpectedStatus {
			c.Status = "down"
			c.ErrorMsg = fmt.Sprintf("expected %d, got %d", m.ExpectedStatus, resp.StatusCode)
		} else if resp.StatusCode >= 400 {
			c.Status = "down"
			c.ErrorMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		} else {
			c.Status = "up"
		}
	}
	return c
}

func (s *Server) maybeAlert(m *store.Monitor, c *store.Check) {
	if c.Status != "down" {
		return
	}
	failures := s.db.ConsecutiveFailures(m.ID)
	rules := s.db.ListAlertRules(m.ID)
	for _, rule := range rules {
		if !rule.Enabled || failures < rule.ConsecutiveFailures {
			continue
		}
		if rule.Type == "webhook" && rule.Endpoint != "" {
			go fireWebhook(rule.Endpoint, m, c, failures)
		}
	}
}

func fireWebhook(url string, m *store.Monitor, c *store.Check, failures int) {
	payload := map[string]any{
		"monitor":   m.Name,
		"url":       m.URL,
		"status":    c.Status,
		"error":     c.ErrorMsg,
		"failures":  failures,
		"resp_ms":   c.RespTimeMs,
		"timestamp": c.CreatedAt,
	}
	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("bellwether: webhook failed for %s: %v", m.Name, err)
		return
	}
	resp.Body.Close()
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func init() { log.SetFlags(log.LstdFlags | log.Lshortfile) }
