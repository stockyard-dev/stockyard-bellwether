package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ db *sql.DB }

type Monitor struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	URL            string            `json:"url"`
	Type           string            `json:"type"` // http, tcp, dns
	IntervalSec    int               `json:"interval_sec"`
	TimeoutSec     int               `json:"timeout_sec"`
	ExpectedStatus int               `json:"expected_status,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Method         string            `json:"method,omitempty"`
	Paused         bool              `json:"paused"`
	CreatedAt      string            `json:"created_at"`
	// computed fields
	Status       string  `json:"status"`        // up, down, unknown
	UptimePct    float64 `json:"uptime_pct"`
	LastCheckAt  string  `json:"last_check_at,omitempty"`
	LastRespMs   int     `json:"last_resp_ms,omitempty"`
	CheckCount   int     `json:"check_count"`
	IncidentCount int    `json:"incident_count"`
}

type Check struct {
	ID         string `json:"id"`
	MonitorID  string `json:"monitor_id"`
	Status     string `json:"status"` // up, down
	RespTimeMs int    `json:"resp_time_ms"`
	StatusCode int    `json:"status_code,omitempty"`
	ErrorMsg   string `json:"error_msg,omitempty"`
	CreatedAt  string `json:"created_at"`
}

type Incident struct {
	ID         string `json:"id"`
	MonitorID  string `json:"monitor_id"`
	StartedAt  string `json:"started_at"`
	ResolvedAt string `json:"resolved_at,omitempty"`
	Cause      string `json:"cause,omitempty"`
	Duration   string `json:"duration,omitempty"`
}

type AlertRule struct {
	ID                  string `json:"id"`
	MonitorID           string `json:"monitor_id"`
	Type                string `json:"type"` // webhook
	Endpoint            string `json:"endpoint"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	Enabled             bool   `json:"enabled"`
	CreatedAt           string `json:"created_at"`
}

func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	dsn := filepath.Join(dataDir, "bellwether.db") + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS monitors (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			type TEXT DEFAULT 'http',
			interval_sec INTEGER DEFAULT 300,
			timeout_sec INTEGER DEFAULT 10,
			expected_status INTEGER DEFAULT 200,
			headers_json TEXT DEFAULT '{}',
			method TEXT DEFAULT 'GET',
			paused INTEGER DEFAULT 0,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS checks (
			id TEXT PRIMARY KEY,
			monitor_id TEXT NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			resp_time_ms INTEGER DEFAULT 0,
			status_code INTEGER DEFAULT 0,
			error_msg TEXT DEFAULT '',
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS incidents (
			id TEXT PRIMARY KEY,
			monitor_id TEXT NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
			started_at TEXT NOT NULL,
			resolved_at TEXT DEFAULT '',
			cause TEXT DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS alert_rules (
			id TEXT PRIMARY KEY,
			monitor_id TEXT NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
			type TEXT DEFAULT 'webhook',
			endpoint TEXT NOT NULL,
			consecutive_failures INTEGER DEFAULT 3,
			enabled INTEGER DEFAULT 1,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_checks_monitor ON checks(monitor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_checks_created ON checks(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_incidents_monitor ON incidents(monitor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_monitor ON alert_rules(monitor_id)`,
	} {
		if _, err := db.Exec(q); err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error { return d.db.Close() }
func genID() string        { return fmt.Sprintf("%d", time.Now().UnixNano()) }
func now() string          { return time.Now().UTC().Format(time.RFC3339) }

// ── Monitors ──

func (d *DB) CreateMonitor(m *Monitor) error {
	m.ID = genID()
	m.CreatedAt = now()
	if m.Type == "" {
		m.Type = "http"
	}
	if m.IntervalSec <= 0 {
		m.IntervalSec = 300
	}
	if m.TimeoutSec <= 0 {
		m.TimeoutSec = 10
	}
	if m.ExpectedStatus <= 0 {
		m.ExpectedStatus = 200
	}
	if m.Method == "" {
		m.Method = "GET"
	}
	if m.Headers == nil {
		m.Headers = map[string]string{}
	}
	hj, _ := json.Marshal(m.Headers)
	paused := 0
	if m.Paused {
		paused = 1
	}
	_, err := d.db.Exec(`INSERT INTO monitors (id,name,url,type,interval_sec,timeout_sec,expected_status,headers_json,method,paused,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.Name, m.URL, m.Type, m.IntervalSec, m.TimeoutSec, m.ExpectedStatus, string(hj), m.Method, paused, m.CreatedAt)
	return err
}

func (d *DB) hydrateMonitor(m *Monitor) {
	// last check
	var lastStatus, lastAt string
	var lastMs int
	if err := d.db.QueryRow(`SELECT status, resp_time_ms, created_at FROM checks WHERE monitor_id=? ORDER BY created_at DESC LIMIT 1`, m.ID).Scan(&lastStatus, &lastMs, &lastAt); err == nil {
		m.Status = lastStatus
		m.LastCheckAt = lastAt
		m.LastRespMs = lastMs
	} else {
		m.Status = "unknown"
	}
	// uptime %
	var total, up int
	d.db.QueryRow(`SELECT COUNT(*) FROM checks WHERE monitor_id=?`, m.ID).Scan(&total)
	d.db.QueryRow(`SELECT COUNT(*) FROM checks WHERE monitor_id=? AND status='up'`, m.ID).Scan(&up)
	m.CheckCount = total
	if total > 0 {
		m.UptimePct = float64(up) / float64(total) * 100
	} else {
		m.UptimePct = 100
	}
	d.db.QueryRow(`SELECT COUNT(*) FROM incidents WHERE monitor_id=?`, m.ID).Scan(&m.IncidentCount)
}

func (d *DB) GetMonitor(id string) *Monitor {
	var m Monitor
	var hj string
	var paused int
	if err := d.db.QueryRow(`SELECT id,name,url,type,interval_sec,timeout_sec,expected_status,headers_json,method,paused,created_at FROM monitors WHERE id=?`, id).Scan(&m.ID, &m.Name, &m.URL, &m.Type, &m.IntervalSec, &m.TimeoutSec, &m.ExpectedStatus, &hj, &m.Method, &paused, &m.CreatedAt); err != nil {
		return nil
	}
	json.Unmarshal([]byte(hj), &m.Headers)
	m.Paused = paused == 1
	d.hydrateMonitor(&m)
	return &m
}

func (d *DB) ListMonitors() []Monitor {
	rows, err := d.db.Query(`SELECT id,name,url,type,interval_sec,timeout_sec,expected_status,headers_json,method,paused,created_at FROM monitors ORDER BY name ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Monitor
	for rows.Next() {
		var m Monitor
		var hj string
		var paused int
		if err := rows.Scan(&m.ID, &m.Name, &m.URL, &m.Type, &m.IntervalSec, &m.TimeoutSec, &m.ExpectedStatus, &hj, &m.Method, &paused, &m.CreatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(hj), &m.Headers)
		m.Paused = paused == 1
		d.hydrateMonitor(&m)
		out = append(out, m)
	}
	return out
}

func (d *DB) UpdateMonitor(id string, m *Monitor) error {
	hj, _ := json.Marshal(m.Headers)
	paused := 0
	if m.Paused {
		paused = 1
	}
	_, err := d.db.Exec(`UPDATE monitors SET name=?,url=?,type=?,interval_sec=?,timeout_sec=?,expected_status=?,headers_json=?,method=?,paused=? WHERE id=?`,
		m.Name, m.URL, m.Type, m.IntervalSec, m.TimeoutSec, m.ExpectedStatus, string(hj), m.Method, paused, id)
	return err
}

func (d *DB) DeleteMonitor(id string) error {
	d.db.Exec(`DELETE FROM checks WHERE monitor_id=?`, id)
	d.db.Exec(`DELETE FROM incidents WHERE monitor_id=?`, id)
	d.db.Exec(`DELETE FROM alert_rules WHERE monitor_id=?`, id)
	_, err := d.db.Exec(`DELETE FROM monitors WHERE id=?`, id)
	return err
}

// ── Checks ──

func (d *DB) RecordCheck(c *Check) error {
	c.ID = genID()
	c.CreatedAt = now()
	_, err := d.db.Exec(`INSERT INTO checks (id,monitor_id,status,resp_time_ms,status_code,error_msg,created_at) VALUES (?,?,?,?,?,?,?)`,
		c.ID, c.MonitorID, c.Status, c.RespTimeMs, c.StatusCode, c.ErrorMsg, c.CreatedAt)
	if err != nil {
		return err
	}
	// incident management
	if c.Status == "down" {
		var openID string
		err := d.db.QueryRow(`SELECT id FROM incidents WHERE monitor_id=? AND resolved_at='' LIMIT 1`, c.MonitorID).Scan(&openID)
		if err == sql.ErrNoRows {
			d.db.Exec(`INSERT INTO incidents (id,monitor_id,started_at,cause) VALUES (?,?,?,?)`,
				genID(), c.MonitorID, c.CreatedAt, c.ErrorMsg)
		}
	} else if c.Status == "up" {
		d.db.Exec(`UPDATE incidents SET resolved_at=? WHERE monitor_id=? AND resolved_at=''`, c.CreatedAt, c.MonitorID)
	}
	// prune old checks (keep last 1000 per monitor)
	d.db.Exec(`DELETE FROM checks WHERE monitor_id=? AND id NOT IN (SELECT id FROM checks WHERE monitor_id=? ORDER BY created_at DESC LIMIT 1000)`, c.MonitorID, c.MonitorID)
	return nil
}

func (d *DB) ListChecks(monitorID string, limit int) []Check {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.db.Query(`SELECT id,monitor_id,status,resp_time_ms,status_code,error_msg,created_at FROM checks WHERE monitor_id=? ORDER BY created_at DESC LIMIT ?`, monitorID, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Check
	for rows.Next() {
		var c Check
		if err := rows.Scan(&c.ID, &c.MonitorID, &c.Status, &c.RespTimeMs, &c.StatusCode, &c.ErrorMsg, &c.CreatedAt); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}

// ── Uptime ──

func (d *DB) Uptime(monitorID string, hours int) float64 {
	if hours <= 0 {
		hours = 24
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour).UTC().Format(time.RFC3339)
	var total, up int
	d.db.QueryRow(`SELECT COUNT(*) FROM checks WHERE monitor_id=? AND created_at>=?`, monitorID, since).Scan(&total)
	d.db.QueryRow(`SELECT COUNT(*) FROM checks WHERE monitor_id=? AND status='up' AND created_at>=?`, monitorID, since).Scan(&up)
	if total == 0 {
		return 100
	}
	return float64(up) / float64(total) * 100
}

// ── Incidents ──

func (d *DB) ListIncidents(monitorID string, limit int) []Incident {
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT id,monitor_id,started_at,resolved_at,cause FROM incidents`
	var args []any
	if monitorID != "" {
		q += ` WHERE monitor_id=?`
		args = append(args, monitorID)
	}
	q += ` ORDER BY started_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Incident
	for rows.Next() {
		var i Incident
		if err := rows.Scan(&i.ID, &i.MonitorID, &i.StartedAt, &i.ResolvedAt, &i.Cause); err != nil {
			continue
		}
		if i.ResolvedAt != "" {
			start, _ := time.Parse(time.RFC3339, i.StartedAt)
			end, _ := time.Parse(time.RFC3339, i.ResolvedAt)
			dur := end.Sub(start)
			if dur < time.Minute {
				i.Duration = fmt.Sprintf("%ds", int(dur.Seconds()))
			} else if dur < time.Hour {
				i.Duration = fmt.Sprintf("%dm", int(dur.Minutes()))
			} else {
				i.Duration = fmt.Sprintf("%dh%dm", int(dur.Hours()), int(dur.Minutes())%60)
			}
		} else {
			i.Duration = "ongoing"
		}
		out = append(out, i)
	}
	return out
}

// ── Alert Rules ──

func (d *DB) CreateAlertRule(a *AlertRule) error {
	a.ID = genID()
	a.CreatedAt = now()
	if a.Type == "" {
		a.Type = "webhook"
	}
	if a.ConsecutiveFailures <= 0 {
		a.ConsecutiveFailures = 3
	}
	enabled := 0
	if a.Enabled {
		enabled = 1
	}
	_, err := d.db.Exec(`INSERT INTO alert_rules (id,monitor_id,type,endpoint,consecutive_failures,enabled,created_at) VALUES (?,?,?,?,?,?,?)`,
		a.ID, a.MonitorID, a.Type, a.Endpoint, a.ConsecutiveFailures, enabled, a.CreatedAt)
	return err
}

func (d *DB) ListAlertRules(monitorID string) []AlertRule {
	rows, err := d.db.Query(`SELECT id,monitor_id,type,endpoint,consecutive_failures,enabled,created_at FROM alert_rules WHERE monitor_id=? ORDER BY created_at DESC`, monitorID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []AlertRule
	for rows.Next() {
		var a AlertRule
		var enabled int
		if err := rows.Scan(&a.ID, &a.MonitorID, &a.Type, &a.Endpoint, &a.ConsecutiveFailures, &enabled, &a.CreatedAt); err != nil {
			continue
		}
		a.Enabled = enabled == 1
		out = append(out, a)
	}
	return out
}

func (d *DB) DeleteAlertRule(id string) error {
	_, err := d.db.Exec(`DELETE FROM alert_rules WHERE id=?`, id)
	return err
}

// ── Consecutive failures (for alerting) ──

func (d *DB) ConsecutiveFailures(monitorID string) int {
	rows, err := d.db.Query(`SELECT status FROM checks WHERE monitor_id=? ORDER BY created_at DESC LIMIT 10`, monitorID)
	if err != nil {
		return 0
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var s string
		rows.Scan(&s)
		if s == "down" {
			count++
		} else {
			break
		}
	}
	return count
}

// ── Stats ──

type Stats struct {
	Monitors  int `json:"monitors"`
	Up        int `json:"up"`
	Down      int `json:"down"`
	Paused    int `json:"paused"`
	Checks    int `json:"checks"`
	Incidents int `json:"incidents"`
}

func (d *DB) Stats() Stats {
	var s Stats
	d.db.QueryRow(`SELECT COUNT(*) FROM monitors`).Scan(&s.Monitors)
	d.db.QueryRow(`SELECT COUNT(*) FROM monitors WHERE paused=1`).Scan(&s.Paused)
	d.db.QueryRow(`SELECT COUNT(*) FROM checks`).Scan(&s.Checks)
	d.db.QueryRow(`SELECT COUNT(*) FROM incidents`).Scan(&s.Incidents)
	// count up/down by latest check per monitor
	rows, _ := d.db.Query(`SELECT m.id, (SELECT status FROM checks WHERE monitor_id=m.id ORDER BY created_at DESC LIMIT 1) as last_status FROM monitors m WHERE m.paused=0`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id, status string
			rows.Scan(&id, &status)
			if status == "up" {
				s.Up++
			} else if status == "down" {
				s.Down++
			}
		}
	}
	return s
}

// ── Active monitors for checker ──

func (d *DB) ActiveMonitors() []Monitor {
	rows, err := d.db.Query(`SELECT id,name,url,type,interval_sec,timeout_sec,expected_status,headers_json,method,paused,created_at FROM monitors WHERE paused=0`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Monitor
	for rows.Next() {
		var m Monitor
		var hj string
		var paused int
		if err := rows.Scan(&m.ID, &m.Name, &m.URL, &m.Type, &m.IntervalSec, &m.TimeoutSec, &m.ExpectedStatus, &hj, &m.Method, &paused, &m.CreatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(hj), &m.Headers)
		m.Paused = paused == 1
		out = append(out, m)
	}
	return out
}

// ── Last check time for a monitor ──

func (d *DB) LastCheckTime(monitorID string) (time.Time, bool) {
	var t string
	if err := d.db.QueryRow(`SELECT created_at FROM checks WHERE monitor_id=? ORDER BY created_at DESC LIMIT 1`, monitorID).Scan(&t); err != nil {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
