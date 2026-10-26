package domain

import "time"

type Timestamps struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewTimestamps(now time.Time) Timestamps {
	return Timestamps{
		CreatedAt: now,
		UpdatedAt: now,
	}
} // both fields = now

func (t *Timestamps) Touch(now time.Time) {
	t.UpdatedAt = now
} // UpdatedAt = now
