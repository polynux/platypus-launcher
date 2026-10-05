package fixes

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"hydra-launcher/internal/types"
)

type ProtonDBSummary struct {
	Tier       string  `json:"tier"`
	Score      float64 `json:"score"`
	Total      int     `json:"total"`
	Confidence string  `json:"confidence"`
	BestReport struct {
		Notes string `json:"notes"`
	} `json:"bestReport"`
}

type Service struct {
	client *http.Client
}

func NewService() *Service {
	return &Service{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// FetchProtonDBFix queries ProtonDB summary API for community tier & recommendations.
func (s *Service) FetchProtonDBFix(appID string) (*types.GameFixInfo, error) {
	if appID == "" {
		return nil, fmt.Errorf("appID cannot be empty")
	}

	url := fmt.Sprintf("https://www.protondb.com/api/v1/reports/summaries/%s.json", appID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "HydraLauncher/1.0 (Linux; x86_64)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("protondb returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var summary ProtonDBSummary
	if err := json.Unmarshal(body, &summary); err != nil {
		return nil, err
	}

	return &types.GameFixInfo{
		Source:      "protondb",
		Tier:        summary.Tier,
		Description: fmt.Sprintf("ProtonDB Tier: %s (Confidence: %s, %d reports)", summary.Tier, summary.Confidence, summary.Total),
		LaunchArgs:  summary.BestReport.Notes,
	}, nil
}

// GetCuratedFix now lives in data plugins (internal/dataplug): the built-in
// recipes and any user-installed ones load through the same path.
