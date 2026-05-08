package tcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type SyncState struct {
	Status           string    `json:"status"`
	ServerURL        string    `json:"server_url"`
	SessionID        string    `json:"session_id"`
	ConnectedAt      time.Time `json:"connected_at"`
	MessagesSent     int       `json:"messages_sent"`
	MessagesReceived int       `json:"messages_received"`
	LastSyncUpdate   string    `json:"last_sync_update"`
}

var stateFile = "data/sync_state.json"

func LoadState() SyncState {
	var state SyncState
	data, err := os.ReadFile(stateFile)
	if err == nil {
		json.Unmarshal(data, &state)
	}
	return state
}

func SaveState(state SyncState) error {
	os.MkdirAll(filepath.Dir(stateFile), 0755)
	data, _ := json.MarshalIndent(state, "", "  ")
	return os.WriteFile(stateFile, data, 0644)
}

func IncrementSent(mangaID string, chapter int) {
	state := LoadState()
	if state.Status == "Active" {
		state.MessagesSent++
		state.LastSyncUpdate = mangaID + " ch. " + string(rune(chapter))
		SaveState(state)
	}
}

func IncrementReceived() {
	state := LoadState()
	if state.Status == "Active" {
		state.MessagesReceived++
		SaveState(state)
	}
}
