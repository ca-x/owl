package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"owl/backend/internal/models"
)

const fileName = "app-settings.json"

type Service struct {
	path string
	mu   sync.RWMutex
	data models.SystemSettings
}

func NewService(dataDir string, defaults models.SystemSettings) (*Service, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("settings data dir is required")
	}
	s := &Service{path: filepath.Join(dataDir, fileName), data: defaults}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) Get(context.Context) models.SystemSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

func (s *Service) Update(ctx context.Context, next models.SystemSettings) (models.SystemSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = normalize(next)
	if err := s.save(ctx); err != nil {
		return models.SystemSettings{}, err
	}
	return s.data, nil
}

func (s *Service) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read settings: %w", err)
	}
	stored := s.data
	if err := json.Unmarshal(data, &stored); err != nil {
		return fmt.Errorf("decode settings: %w", err)
	}
	s.data = normalize(stored)
	return nil
}

func (s *Service) save(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write settings temp file: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace settings file: %w", err)
	}
	return nil
}

func normalize(settings models.SystemSettings) models.SystemSettings {
	settings.FooterExtra = strings.TrimSpace(settings.FooterExtra)
	settings.Copyright = strings.TrimSpace(settings.Copyright)
	return settings
}
