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
        status TEXT DEFAULT 'todo'
	);`
	
	_, err = DB.Exec(query)

    DB.Exec("ALTER TABLE notes ADD COLUMN status TEXT DEFAULT 'todo'")

	return err
}

func AddNote(content string, deadline string) error {
    if deadline == "" {
        _, err := DB.Exec("INSERT INTO notes (content) VALUES (?)", content)
        return err
    }
    _, err := DB.Exec("INSERT INTO notes (content, deadline) VALUES (?, ?)", content, deadline)
    return err
}

func DeleteNote(id int) error {
	_, err := DB.Exec("DELETE FROM notes WHERE id = ?", id)
	return err
}

func GetAllNotes() ([]models.Note, error) {
	rows, err := DB.Query("SELECT id, content, created_at, deadline, notified, status FROM notes ORDER BY created_at DESC")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var notes []models.Note
    for rows.Next() {
        var n models.Note
        // Используем sql.NullTime для deadline, так как он может быть NULL
        var deadline sql.NullTime
        err := rows.Scan(&n.ID, &n.Content, &n.CreatedAt, &deadline, &n.Notified, &n.Status)
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

func UpdateNote(id int, content string, deadline string, status string, notified bool) error {
	if deadline == "" {
		_, err := DB.Exec("UPDATE notes SET content = ?, deadline = NULL, status = ?, notified = ? WHERE id = ?", content, status, notified, id)
		return err
	}
	_, err := DB.Exec("UPDATE notes SET content = ?, deadline = ?, status = ?, notified = ? WHERE id = ?", content, deadline, status, notified, id)
	return err
}