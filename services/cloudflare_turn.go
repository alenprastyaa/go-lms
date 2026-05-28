package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultCloudflareTurnAPIURL = "https://rtc.live.cloudflare.com/v1/turn/keys"

type cloudflareTurnICEserver struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

type cloudflareTurnGenerateResponse struct {
	ICEServers      []cloudflareTurnICEserver `json:"iceServers"`
	ICEServersSnake []cloudflareTurnICEserver `json:"ice_servers"`
	Result          *struct {
		ICEServers      []cloudflareTurnICEserver `json:"iceServers"`
		ICEServersSnake []cloudflareTurnICEserver `json:"ice_servers"`
		TTL             int                       `json:"ttl,omitempty"`
		ExpiresAt       string                    `json:"expiresAt,omitempty"`
		ExpiresAtSnake  string                    `json:"expires_at,omitempty"`
	} `json:"result,omitempty"`
	TTL            int    `json:"ttl,omitempty"`
	ExpiresAt      string `json:"expiresAt,omitempty"`
	ExpiresAtSnake string `json:"expires_at,omitempty"`
	Error          *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func cloudflareTurnKeyID() string {
	return strings.TrimSpace(os.Getenv("CLOUDFLARE_TURN_KEY_ID"))
}

func cloudflareTurnAPIToken() string {
	return strings.TrimSpace(os.Getenv("CLOUDFLARE_TURN_API_TOKEN"))
}

func cloudflareTurnAPIURL(keyID string) string {
	baseURL := strings.TrimSpace(os.Getenv("CLOUDFLARE_TURN_API_URL"))
	if baseURL == "" {
		baseURL = defaultCloudflareTurnAPIURL
	}
	return fmt.Sprintf("%s/%s/credentials/generate-ice-servers", strings.TrimRight(baseURL, "/"), keyID)
}

func staticCloudflareTurnICEServers() ([]cloudflareTurnICEserver, error) {
	rawJSON := strings.TrimSpace(os.Getenv("CLOUDFLARE_TURN_ICE_SERVERS"))
	if rawJSON != "" {
		var parsed []cloudflareTurnICEserver
		if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
			return nil, fmt.Errorf("CLOUDFLARE_TURN_ICE_SERVERS tidak valid: %w", err)
		}
		return parsed, nil
	}

	username := strings.TrimSpace(os.Getenv("CLOUDFLARE_TURN_USERNAME"))
	credential := strings.TrimSpace(os.Getenv("CLOUDFLARE_TURN_CREDENTIAL"))
	if credential == "" {
		credential = strings.TrimSpace(os.Getenv("CLOUDFLARE_TURN_PASSWORD"))
	}
	if username == "" || credential == "" {
		return nil, nil
	}

	return []cloudflareTurnICEserver{
		{
			URLs: []string{
				"stun:stun.cloudflare.com:3478",
				"turn:turn.cloudflare.com:3478?transport=udp",
				"turn:turn.cloudflare.com:3478?transport=tcp",
				"turns:turn.cloudflare.com:5349?transport=tcp",
			},
			Username:   username,
			Credential: credential,
		},
	}, nil
}

func normalizeCloudflareTurnResponse(parsed cloudflareTurnGenerateResponse) cloudflareTurnGenerateResponse {
	if len(parsed.ICEServers) == 0 {
		parsed.ICEServers = parsed.ICEServersSnake
	}
	if parsed.Result != nil {
		if len(parsed.ICEServers) == 0 {
			parsed.ICEServers = parsed.Result.ICEServers
		}
		if len(parsed.ICEServers) == 0 {
			parsed.ICEServers = parsed.Result.ICEServersSnake
		}
		if parsed.TTL == 0 {
			parsed.TTL = parsed.Result.TTL
		}
		if strings.TrimSpace(parsed.ExpiresAt) == "" {
			parsed.ExpiresAt = parsed.Result.ExpiresAt
		}
		if strings.TrimSpace(parsed.ExpiresAtSnake) == "" {
			parsed.ExpiresAtSnake = parsed.Result.ExpiresAtSnake
		}
	}
	return parsed
}

func FetchCloudflareTurnICEServers() ([]cloudflareTurnICEserver, time.Time, error) {
	if staticServers, err := staticCloudflareTurnICEServers(); err != nil {
		return nil, time.Time{}, err
	} else if len(staticServers) > 0 {
		return staticServers, time.Now().Add(24 * time.Hour), nil
	}

	keyID := cloudflareTurnKeyID()
	apiToken := cloudflareTurnAPIToken()
	if keyID == "" || apiToken == "" {
		return nil, time.Time{}, fmt.Errorf("Cloudflare TURN belum dikonfigurasi")
	}

	payload := map[string]any{
		"ttl": 86400,
	}
	rawBody, err := json.Marshal(payload)
	if err != nil {
		return nil, time.Time{}, err
	}

	req, err := http.NewRequest(http.MethodPost, cloudflareTurnAPIURL(keyID), bytes.NewReader(rawBody))
	if err != nil {
		return nil, time.Time{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer resp.Body.Close()

	var parsed cloudflareTurnGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, time.Time{}, err
	}
	parsed = normalizeCloudflareTurnResponse(parsed)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
			return nil, time.Time{}, fmt.Errorf("%s", strings.TrimSpace(parsed.Error.Message))
		}
		return nil, time.Time{}, fmt.Errorf("gagal mengambil TURN credentials cloudflare: %s", resp.Status)
	}
	if len(parsed.ICEServers) == 0 {
		return nil, time.Time{}, fmt.Errorf("TURN credentials cloudflare tidak mengembalikan ICE servers")
	}

	expiresAt := time.Time{}
	expiresAtValue := strings.TrimSpace(parsed.ExpiresAt)
	if expiresAtValue == "" {
		expiresAtValue = strings.TrimSpace(parsed.ExpiresAtSnake)
	}
	if expiresAtValue != "" {
		if parsedTime, err := time.Parse(time.RFC3339, expiresAtValue); err == nil {
			expiresAt = parsedTime
		}
	}
	if expiresAt.IsZero() && parsed.TTL > 0 {
		expiresAt = time.Now().Add(time.Duration(parsed.TTL) * time.Second)
	}

	return parsed.ICEServers, expiresAt, nil
}
