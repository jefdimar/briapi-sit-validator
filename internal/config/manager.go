package config

import (
	"sync"
)

// Manager wraps a Config and provides a thread-safe reloadable config container.
type Manager struct {
	mu   sync.RWMutex
	cfg  *Config
	path string
}

// NewManager creates a new Manager loaded from path.
func NewManager(path string) (*Manager, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	return &Manager{cfg: cfg, path: path}, nil
}

// Get returns the current Config.
func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// Path returns the configuration file path.
func (m *Manager) Path() string {
	return m.path
}

// Reload reloads the configuration from path.
func (m *Manager) Reload() error {
	newCfg, err := Load(m.path)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = newCfg
	m.mu.Unlock()
	return nil
}

// NewManagerWithConfig creates a new Manager initialized with a pre-built Config.
// Useful for unit tests where configuration is mocked in-memory.
func NewManagerWithConfig(cfg *Config) *Manager {
	return &Manager{cfg: cfg}
}
