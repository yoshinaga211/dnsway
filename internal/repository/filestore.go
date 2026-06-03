package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStore provides JSON-file persistence for mock mode.
// Data survives restarts without requiring PostgreSQL.
type FileStore struct {
	path string
	mu   sync.RWMutex
	Data FileStoreData
}

type FileStoreData struct {
	Categories    map[string]map[string]bool        `json:"categories"`
	Allowlist     map[string]map[string]bool        `json:"allowlist"`
	Denylist      map[string]map[string]string      `json:"denylist"`
	TimeWindows   []map[string]interface{}          `json:"time_windows"`
	Profiles      map[string]map[string]interface{} `json:"profiles"`
	Users         []map[string]interface{}          `json:"users"`
	Subscriptions []map[string]interface{}          `json:"subscriptions"`
	Payments      []map[string]interface{}          `json:"payments"`
	Plans         []map[string]interface{}          `json:"plans"`
}

func NewFileStore(dataDir string) *FileStore {
	os.MkdirAll(dataDir, 0755)
	fs := &FileStore{
		path: filepath.Join(dataDir, "state.json"),
		Data: FileStoreData{
			Categories:    map[string]map[string]bool{},
			Allowlist:     map[string]map[string]bool{},
			Denylist:      map[string]map[string]string{},
			TimeWindows:   []map[string]interface{}{},
			Profiles:      map[string]map[string]interface{}{},
			Users:         []map[string]interface{}{},
			Subscriptions: []map[string]interface{}{},
			Payments:      []map[string]interface{}{},
			Plans:         []map[string]interface{}{},
		},
	}
	fs.Load()
	return fs
}

func (fs *FileStore) Load() {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	data, err := os.ReadFile(fs.path)
	if err != nil {
		return
	}

	var stored FileStoreData
	if err := json.Unmarshal(data, &stored); err != nil {
		fmt.Printf("Failed to load state file: %v\n", err)
		return
	}

	if stored.Categories != nil {
		fs.Data.Categories = stored.Categories
	}
	if stored.Allowlist != nil {
		fs.Data.Allowlist = stored.Allowlist
	}
	if stored.Denylist != nil {
		fs.Data.Denylist = stored.Denylist
	}
	if stored.TimeWindows != nil {
		fs.Data.TimeWindows = stored.TimeWindows
	}
	if stored.Profiles != nil {
		fs.Data.Profiles = stored.Profiles
	}
	if stored.Users != nil {
		fs.Data.Users = stored.Users
	}
	if stored.Subscriptions != nil {
		fs.Data.Subscriptions = stored.Subscriptions
	}
	if stored.Payments != nil {
		fs.Data.Payments = stored.Payments
	}
	if stored.Plans != nil {
		fs.Data.Plans = stored.Plans
	}
	fmt.Printf("Loaded persisted state (%d profiles, %d time windows)\n",
		len(fs.Data.Profiles), len(fs.Data.TimeWindows))
}

func (fs *FileStore) Save() {
	fs.mu.RLock()
	data, err := json.MarshalIndent(fs.Data, "", "  ")
	fs.mu.RUnlock()

	if err != nil {
		fmt.Printf("Failed to marshal state: %v\n", err)
		return
	}

	tmpPath := fs.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		fmt.Printf("Failed to write state file: %v\n", err)
		return
	}
	os.Rename(tmpPath, fs.path)
}
