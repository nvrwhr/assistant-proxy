package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore implements Memory using a SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`create table if not exists messages (id integer primary key autoincrement, thread text, content text, created_at integer)`); err != nil {
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) SaveMessage(threadID string, msg string) error {
	_, err := s.db.Exec(`insert into messages(thread, content, created_at) values(?,?,strftime('%s','now'))`, threadID, msg)
	return err
}

func (s *SQLiteStore) GetMessages(threadID string) ([]string, error) {
	rows, err := s.db.Query(`select content from messages where thread=? order by id`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []string
	for rows.Next() {
		var msg string
		if err := rows.Scan(&msg); err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}
