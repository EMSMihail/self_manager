package db

import (
	"database/sql"
	"self-manager/models"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB(filepath string) error {
	var err error
	DB, err = sql.Open("sqlite3", filepath)
	if err != nil {
		return err
	}

	// Создаем таблицу, если она не существует
	query := `
	CREATE TABLE IF NOT EXISTS notes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		deadline DATETIME,
    	notified BOOLEAN DEFAULT 0,
        status TEXT DEFAULT 'todo',
        priority TEXT DEFAULT 'medium'
	);`
	
	_, err = DB.Exec(query)

	return err
}

func AddNote(content string, deadline string, priority string) error {
	if priority == "" {
		priority = "medium"
	}
	if deadline == "" {
		_, err := DB.Exec("INSERT INTO notes (content, priority) VALUES (?, ?)", content, priority)
		return err
	}
	_, err := DB.Exec("INSERT INTO notes (content, deadline, priority) VALUES (?, ?, ?)", content, deadline, priority)
	return err
}

func DeleteNote(id int) error {
	_, err := DB.Exec("DELETE FROM notes WHERE id = ?", id)
	return err
}

func GetAllNotes() ([]models.Note, error) {
	rows, err := DB.Query("SELECT id, content, created_at, deadline, notified, status, priority FROM notes ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []models.Note
	for rows.Next() {
		var n models.Note
		var deadline sql.NullTime
		err := rows.Scan(&n.ID, &n.Content, &n.CreatedAt, &deadline, &n.Notified, &n.Status, &n.Priority)
		if err != nil {
			return nil, err
		}
		if deadline.Valid {
			n.Deadline = &deadline.Time
		}
		notes = append(notes, n)
	}
	return notes, nil
}

func UpdateNote(id int, content string, deadline string, status string, notified bool, priority string) error {
	if priority == "" {
		priority = "medium"
	}
	if deadline == "" {
		_, err := DB.Exec("UPDATE notes SET content = ?, deadline = NULL, status = ?, notified = ?, priority = ? WHERE id = ?", content, status, notified, priority, id)
		return err
	}
	_, err := DB.Exec("UPDATE notes SET content = ?, deadline = ?, status = ?, notified = ?, priority = ? WHERE id = ?", content, deadline, status, notified, priority, id)
	return err
}

func GetNoteByID(id int) (models.Note, error) {
	var n models.Note
	var deadline sql.NullTime
	
	err := DB.QueryRow("SELECT id, content, created_at, deadline, notified, status, priority FROM notes WHERE id = ?", id).
		Scan(&n.ID, &n.Content, &n.CreatedAt, &deadline, &n.Notified, &n.Status, &n.Priority)
		
	if deadline.Valid {
		n.Deadline = &deadline.Time
	}
	return n, err
}