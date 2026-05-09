package udp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// NotifyState holds the persisted client-side state for UDP notifications.
type NotifyState struct {
	Subscribed  bool      `json:"subscribed"`
	ServerAddr  string    `json:"server_addr"`
	SubscribedAt time.Time `json:"subscribed_at,omitempty"`
	UserID      string    `json:"user_id,omitempty"`
}

var notifyStateFile = "data/notify_state.json"

// LoadNotifyState reads the notification state from disk.
func LoadNotifyState() NotifyState {
	var state NotifyState
	data, err := os.ReadFile(notifyStateFile)
	if err == nil {
		json.Unmarshal(data, &state)
	}
	return state
}

// SaveNotifyState writes the notification state to disk.
func SaveNotifyState(state NotifyState) error {
	os.MkdirAll(filepath.Dir(notifyStateFile), 0755)
	data, _ := json.MarshalIndent(state, "", "  ")
	return os.WriteFile(notifyStateFile, data, 0644)
}
