package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

type SessionType string

const (
	SessionTypeManual     SessionType = "manual"
	SessionTypeAutonomous SessionType = "autonomous"
)

func (s SessionType) String() string {
	return string(s)
}

type SessionOutcome string

const (
	SessionOutcomeRunning     SessionOutcome = "running"
	SessionOutcomeCompleted   SessionOutcome = "completed"
	SessionOutcomeFailed      SessionOutcome = "failed"
	SessionOutcomeInterrupted SessionOutcome = "interrupted"
)

func (s SessionOutcome) String() string {
	return string(s)
}

type SessionRecord struct {
	ID            string         `json:"id"`
	PRNumber      int            `json:"pr_number"`
	Repo          string         `json:"repo"`
	SessionType   SessionType    `json:"session_type"`
	ActionType    string         `json:"action_type"`
	Prompt        string         `json:"prompt"`
	WorkDir       string         `json:"work_dir"`
	WindowName    string         `json:"window_name"`
	HeadCommitOID string         `json:"headCommitOID,omitempty"`
	StartedAt     time.Time      `json:"started_at"`
	EndedAt       *time.Time     `json:"ended_at,omitempty"`
	Outcome       SessionOutcome `json:"outcome"`
	Error         string         `json:"error,omitempty"`
}

func NewSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	return hex.EncodeToString(b)
}
