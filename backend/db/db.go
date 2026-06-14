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
		description TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		deadline DATETIME,
    	notified BOOLEAN DEFAULT 0,
        status TEXT DEFAULT 'todo',
        priority TEXT DEFAULT 'low'
	);`
	
	_, err = DB.Exec(query)
	if err != nil {
	return err
}

	// Миграция для существующей базы: добавляем колонку, если её не было.
	// Ошибку игнорируем, так как если колонка уже есть, SQLite выдаст ошибку "duplicate column name"
	_, _ = DB.Exec("ALTER TABLE notes ADD COLUMN description TEXT DEFAULT '';")

	return nil
}

func AddNote(content string, description string, deadline string, priority string) (int64, error) {
    if priority == "" {
        priority = "low"
    }
    
    var res sql.Result
    var err error

    if deadline == "" {
		res, err = DB.Exec("INSERT INTO notes (content, description, priority) VALUES (?, ?, ?)", content, description, priority)
    } else {
		res, err = DB.Exec("INSERT INTO notes (content, description, deadline, priority) VALUES (?, ?, ?, ?)", content, description, deadline, priority)
    }

    if err != nil {
        return 0, err
    }

    id, err := res.LastInsertId()
    return id, err
}

func DeleteNote(id int) error {
	_, err := DB.Exec("DELETE FROM notes WHERE id = ?", id)
	return err
}

func GetAllNotes() ([]models.Note, error) {
	rows, err := DB.Query("SELECT id, content, description, created_at, deadline, notified, status, priority FROM notes ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []models.Note
	for rows.Next() {
		var n models.Note
		var deadline sql.NullTime
		err := rows.Scan(&n.ID, &n.Content, &n.Description, &n.CreatedAt, &deadline, &n.Notified, &n.Status, &n.Priority)
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

func UpdateNote(id int, content string, description string, deadline string, status string, notified bool, priority string) error {
	if priority == "" {
		priority = "low"
	}
	if deadline == "" {
		_, err := DB.Exec("UPDATE notes SET content = ?, description = ?, deadline = NULL, status = ?, notified = ?, priority = ? WHERE id = ?", content, description, status, notified, priority, id)
		return err
	}
	_, err := DB.Exec("UPDATE notes SET content = ?, description = ?, deadline = ?, status = ?, notified = ?, priority = ? WHERE id = ?", content, description, deadline, status, notified, priority, id)
	return err
}

func GetNoteByID(id int) (models.Note, error) {
	var n models.Note
	var deadline sql.NullTime
	
	err := DB.QueryRow("SELECT id, content, description, created_at, deadline, notified, status, priority FROM notes WHERE id = ?", id).
		Scan(&n.ID, &n.Content, &n.Description, &n.CreatedAt, &deadline, &n.Notified, &n.Status, &n.Priority)
		
	if deadline.Valid {
		n.Deadline = &deadline.Time
	}
	return n, err
}