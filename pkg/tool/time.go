package tool

import "time"

// RoundToDate округляет дату в t до круглого дня
func RoundToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
