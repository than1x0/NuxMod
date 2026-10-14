package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Sathira-Laksahan/vless-linux/internal/parser"
)

const (
	ConfigDir       = "/etc/nuxmod"
	ConfigFile      = "profiles.json"
	LegacyConfigDir = "/etc/vless-linux"
)

// Profile represents a saved VLESS connection profile.
type Profile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Server    string    `json:"server"`
	Port      int       `json:"port"`
	SNI       string    `json:"sni"`
	CreatedAt time.Time `json:"created_at"`
	Active    bool      `json:"active"`
}

type Store struct {
	ActiveID string    `json:"active_id"`
	Profiles []Profile `json:"profiles"`
}

type Manager struct {
	mu       sync.RWMutex
	filePath string
	store    Store
}

func getStorePath() string {
	if os.Geteuid() == 0 {
		_ = os.MkdirAll(ConfigDir, 0755)
		newPath := filepath.Join(ConfigDir, ConfigFile)
		oldPath := filepath.Join(LegacyConfigDir, ConfigFile)
		// Auto-migrate from legacy config if new file doesn't exist yet
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			if oldData, err := os.ReadFile(oldPath); err == nil {
				_ = os.WriteFile(newPath, oldData, 0644)
			}
		}
		return newPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".nuxmod")
	_ = os.MkdirAll(dir, 0755)
	newPath := filepath.Join(dir, ConfigFile)
	oldPath := filepath.Join(home, ".vless-linux", ConfigFile)
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		if oldData, err := os.ReadFile(oldPath); err == nil {
			_ = os.WriteFile(newPath, oldData, 0644)
		}
	}
	return newPath
}

func NewManager() (*Manager, error) {
	m := &Manager{
		filePath: getStorePath(),
	}
	if err := m.load(); err != nil {
		// If file doesn't exist, start fresh
		m.store = Store{Profiles: []Profile{}}
		_ = m.save()
	}
	return m, nil
}

func (m *Manager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &m.store)
}

func (m *Manager) save() error {
	data, err := json.MarshalIndent(m.store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0644)
}

// Add adds a new VLESS link as a profile.
func (m *Manager) Add(rawURL string, customName string) (*Profile, error) {
	cfg, err := parser.ParseVlessLink(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid VLESS link: %w", err)
	}

	name := customName
	if name == "" {
		if cfg.Remarks != "" {
			name = cfg.Remarks
		} else {
			name = fmt.Sprintf("Node-%s:%d", cfg.Server, cfg.Port)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already exists by URL
	for i, p := range m.store.Profiles {
		if p.URL == rawURL {
			m.store.Profiles[i].Name = name
			_ = m.save()
			return &m.store.Profiles[i], nil
		}
	}

	id := fmt.Sprintf("p_%d", time.Now().UnixNano()%100000)
	isFirst := len(m.store.Profiles) == 0

	newProf := Profile{
		ID:        id,
		Name:      name,
		URL:       rawURL,
		Server:    cfg.Server,
		Port:      cfg.Port,
		SNI:       cfg.SNI,
		CreatedAt: time.Now(),
		Active:    isFirst,
	}

	m.store.Profiles = append(m.store.Profiles, newProf)
	if isFirst {
		m.store.ActiveID = id
	}

	if err := m.save(); err != nil {
		return nil, fmt.Errorf("failed to save profile: %w", err)
	}
	return &newProf, nil
}

// Delete removes a profile by ID or Name.
func (m *Manager) Delete(idOrName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	found := false
	var updated []Profile
	for _, p := range m.store.Profiles {
		if p.ID == idOrName || p.Name == idOrName {
			found = true
			continue
		}
		updated = append(updated, p)
	}

	if !found {
		return fmt.Errorf("profile '%s' not found", idOrName)
	}

	m.store.Profiles = updated
	// If active was deleted, pick the first one
	if m.store.ActiveID == idOrName || len(m.store.Profiles) > 0 {
		if len(m.store.Profiles) > 0 {
			m.store.ActiveID = m.store.Profiles[0].ID
			m.store.Profiles[0].Active = true
		} else {
			m.store.ActiveID = ""
		}
	}

	return m.save()
}

// SetActive sets a profile as the currently active profile.
func (m *Manager) SetActive(idOrName string) (*Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var selected *Profile
	for i := range m.store.Profiles {
		if m.store.Profiles[i].ID == idOrName || m.store.Profiles[i].Name == idOrName {
			m.store.Profiles[i].Active = true
			m.store.ActiveID = m.store.Profiles[i].ID
			selected = &m.store.Profiles[i]
		} else {
			m.store.Profiles[i].Active = false
		}
	}

	if selected == nil {
		return nil, fmt.Errorf("profile '%s' not found", idOrName)
	}

	if err := m.save(); err != nil {
		return nil, err
	}
	return selected, nil
}

// GetActive returns the active profile.
func (m *Manager) GetActive() (*Profile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.store.Profiles) == 0 {
		return nil, fmt.Errorf("no profiles found")
	}

	for _, p := range m.store.Profiles {
		if p.ID == m.store.ActiveID || p.Active {
			return &p, nil
		}
	}

	// Fallback to first profile
	return &m.store.Profiles[0], nil
}

// List returns all saved profiles.
func (m *Manager) List() []Profile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Profile, len(m.store.Profiles))
	copy(result, m.store.Profiles)
	return result
}
