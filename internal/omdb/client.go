package omdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://www.omdbapi.com/"

type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

func New(apiKey string) *Client {
	return NewWithBaseURL(apiKey, defaultBaseURL)
}

func NewWithBaseURL(apiKey, baseURL string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    baseURL,
	}
}

type Ratings struct {
	IMDb           string
	RottenTomatoes string
}

func (c *Client) Ratings(ctx context.Context, imdbID string) (*Ratings, error) {
	q := url.Values{}
	q.Set("apikey", c.apiKey)
	q.Set("i", imdbID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("omdb: status %d", resp.StatusCode)
	}

	var out struct {
		Response string `json:"Response"`
		Ratings  []struct {
			Source string `json:"Source"`
			Value  string `json:"Value"`
		} `json:"Ratings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Response != "True" {
		return &Ratings{}, nil
	}

	r := &Ratings{}
	for _, rating := range out.Ratings {
		switch rating.Source {
		case "Internet Movie Database":
			r.IMDb = rating.Value
		case "Rotten Tomatoes":
			r.RottenTomatoes = rating.Value
		}
	}
	return r, nil
}
