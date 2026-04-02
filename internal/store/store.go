package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct { db *sql.DB }

type Monitor struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	URL          string   `json:"url"`
	Interval     string   `json:"interval"`
	Status       string   `json:"status"`
	LastCheck    string   `json:"last_check"`
	CreatedAt    string   `json:"created_at"`
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
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS monitors (
			id TEXT PRIMARY KEY,\n\t\t\tname TEXT DEFAULT '',\n\t\t\turl TEXT DEFAULT '',\n\t\t\tinterval TEXT DEFAULT '60s',\n\t\t\tstatus TEXT DEFAULT 'unknown',\n\t\t\tlast_check TEXT DEFAULT '',
			created_at TEXT DEFAULT (datetime('now'))
		)`)
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error { return d.db.Close() }

func genID() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

func (d *DB) Create(e *Monitor) error {
	e.ID = genID()
	e.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	_, err := d.db.Exec(`INSERT INTO monitors (id, name, url, interval, status, last_check, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Name, e.URL, e.Interval, e.Status, e.LastCheck, e.CreatedAt)
	return err
}

func (d *DB) Get(id string) *Monitor {
	row := d.db.QueryRow(`SELECT id, name, url, interval, status, last_check, created_at FROM monitors WHERE id=?`, id)
	var e Monitor
	if err := row.Scan(&e.ID, &e.Name, &e.URL, &e.Interval, &e.Status, &e.LastCheck, &e.CreatedAt); err != nil {
		return nil
	}
	return &e
}

func (d *DB) List() []Monitor {
	rows, err := d.db.Query(`SELECT id, name, url, interval, status, last_check, created_at FROM monitors ORDER BY created_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []Monitor
	for rows.Next() {
		var e Monitor
		if err := rows.Scan(&e.ID, &e.Name, &e.URL, &e.Interval, &e.Status, &e.LastCheck, &e.CreatedAt); err != nil {
			continue
		}
		result = append(result, e)
	}
	return result
}

func (d *DB) Delete(id string) error {
	_, err := d.db.Exec(`DELETE FROM monitors WHERE id=?`, id)
	return err
}

func (d *DB) Count() int {
	var n int
	d.db.QueryRow(`SELECT COUNT(*) FROM monitors`).Scan(&n)
	return n
}
