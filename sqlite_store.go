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
	if _, err := db.Exec(`create table if not exists messages (id integer primary key autoincrement, thread text, role text, content text, created_at integer)`); err != nil {
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) SaveMessage(threadID string, msg Message) error {
	_, err := s.db.Exec(`insert into messages(thread, role, content, created_at) values(?,?,?,strftime('%s','now'))`, threadID, msg.Role, msg.Content)
	return err
}

func (s *SQLiteStore) GetMessages(threadID string) ([]Message, error) {
	rows, err := s.db.Query(`select role, content from messages where thread=? order by id`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.Role, &m.Content); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}
