package anime

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Quote represents an anime quote
type Quote struct {
	Anime     string `json:"anime"`
	Character string `json:"character"`
	Quote     string `json:"quote"`
}

// Service manages anime quotes
type Service struct {
	quotes []Quote
	mu     sync.RWMutex
	rng    *rand.Rand
}

// NewService creates a new anime service from embedded JSON data
func NewService(jsonData []byte) (*Service, error) {
	var quotes []Quote
	if err := json.Unmarshal(jsonData, &quotes); err != nil {
		return nil, fmt.Errorf("failed to parse anime quotes: %w", err)
	}

	return &Service{
		quotes: quotes,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

// GetRandomQuote returns a random quote
func (s *Service) GetRandomQuote() interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.quotes) == 0 {
		return map[string]string{
			"error": "no quotes available",
		}
	}

	idx := s.rng.Intn(len(s.quotes))
	q := s.quotes[idx]

	return map[string]interface{}{
		"anime":     q.Anime,
		"character": q.Character,
		"quote":     q.Quote,
	}
}

// GetAllQuotes returns all quotes
func (s *Service) GetAllQuotes() interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]interface{}, len(s.quotes))
	for i, q := range s.quotes {
		result[i] = map[string]interface{}{
			"anime":     q.Anime,
			"character": q.Character,
			"quote":     q.Quote,
		}
	}
	return result
}

// GetTotalQuotes returns the total number of quotes
func (s *Service) GetTotalQuotes() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.quotes)
}
