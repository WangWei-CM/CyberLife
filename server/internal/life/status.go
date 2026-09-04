package life

import "time"

type Status string

const (
	StatusActive  Status = "active"
	StatusLost    Status = "lost"
	StatusStopped Status = "stopped"
)

// DeriveStatus is deterministic and uses writer login activity as the heartbeat.
func DeriveStatus(lastActive, now time.Time, current Status) Status {
	if current == StatusStopped {
		return StatusStopped
	}
	age := now.Sub(lastActive)
	if age >= 30*24*time.Hour {
		return StatusStopped
	}
	if age >= 7*24*time.Hour {
		return StatusLost
	}
	return StatusActive
}
