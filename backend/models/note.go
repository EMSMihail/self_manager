package models

import "time"

type Note struct {
	ID        int       `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Deadline  *time.Time `json:"deadline"`
    Notified  bool      `json:"notified"`
	Status    string     `json:"status"`
	Priority  string     `json:"priority"`
}