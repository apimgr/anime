package anime

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"strings"
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
	quote, ok := s.GetRandomQuoteRecord()
	if !ok { return map[string]string{"error": "no quotes available"} }
	return map[string]interface{}{"anime": quote.Anime, "character": quote.Character, "quote": quote.Quote}
}

// GetRandomQuoteRecord returns a random typed quote and reports an empty dataset.
func (s *Service) GetRandomQuoteRecord() (Quote, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.quotes) == 0 { return Quote{}, false }
	return s.quotes[s.rng.Intn(len(s.quotes))], true
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


// SearchQuotes returns quotes matching the supplied case-insensitive filters.
func (s *Service) SearchQuotes(query, animeName, character string) []Quote {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query = strings.ToLower(strings.TrimSpace(query))
	animeName = strings.ToLower(strings.TrimSpace(animeName))
	character = strings.ToLower(strings.TrimSpace(character))
	results := make([]Quote, 0)
	for _, quote := range s.quotes {
		if query != "" && !strings.Contains(strings.ToLower(quote.Quote), query) && !strings.Contains(strings.ToLower(quote.Anime), query) && !strings.Contains(strings.ToLower(quote.Character), query) {
			continue
		}
		if animeName != "" && !strings.Contains(strings.ToLower(quote.Anime), animeName) {
			continue
		}
		if character != "" && !strings.Contains(strings.ToLower(quote.Character), character) {
			continue
		}
		results = append(results, quote)
	}
	return results
}
