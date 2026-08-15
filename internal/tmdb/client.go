package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.themoviedb.org/3"

type Client struct {
	token      string
	httpClient *http.Client
	baseURL    string
}

func New(token string) *Client {
	return NewWithBaseURL(token, defaultBaseURL)
}

func NewWithBaseURL(token, baseURL string) *Client {
	return &Client{
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    baseURL,
	}
}

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (c *Client) Genres(ctx context.Context) ([]Genre, error) {
	var out struct {
		Genres []Genre `json:"genres"`
	}
	if err := c.get(ctx, "/genre/movie/list", nil, &out); err != nil {
		return nil, err
	}
	return out.Genres, nil
}

type DiscoverFilters struct {
	Page      int
	YearFrom  int
	YearTo    int
	GenreIDs  []int
	MinRating float64
	MinVotes  int
}

type DiscoverResult struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Overview    string  `json:"overview"`
	PosterPath  string  `json:"poster_path"`
	ReleaseDate string  `json:"release_date"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
	GenreIDs    []int   `json:"genre_ids"`
}

type discoverResponse struct {
	Page       int              `json:"page"`
	Results    []DiscoverResult `json:"results"`
	TotalPages int              `json:"total_pages"`
}

func (c *Client) Discover(ctx context.Context, f DiscoverFilters) (results []DiscoverResult, totalPages int, err error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(f.Page))
	q.Set("sort_by", "popularity.desc")
	q.Set("include_adult", "false")
	if f.YearFrom > 0 {
		q.Set("primary_release_date.gte", fmt.Sprintf("%d-01-01", f.YearFrom))
	}
	if f.YearTo > 0 {
		q.Set("primary_release_date.lte", fmt.Sprintf("%d-12-31", f.YearTo))
	}
	if len(f.GenreIDs) > 0 {
		q.Set("with_genres", joinInts(f.GenreIDs))
	}
	if f.MinRating > 0 {
		q.Set("vote_average.gte", strconv.FormatFloat(f.MinRating, 'f', 1, 64))
	}
	if f.MinVotes > 0 {
		q.Set("vote_count.gte", strconv.Itoa(f.MinVotes))
	}

	var out discoverResponse
	if err := c.get(ctx, "/discover/movie", q, &out); err != nil {
		return nil, 0, err
	}
	return out.Results, out.TotalPages, nil
}

type MovieDetail struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Overview    string  `json:"overview"`
	PosterPath  string  `json:"poster_path"`
	ReleaseDate string  `json:"release_date"`
	Runtime     int     `json:"runtime"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
	Genres      []Genre `json:"genres"`
	Videos      struct {
		Results []Video `json:"results"`
	} `json:"videos"`
	ExternalIDs struct {
		IMDbID string `json:"imdb_id"`
	} `json:"external_ids"`
}

type Video struct {
	Key  string `json:"key"`
	Site string `json:"site"`
	Type string `json:"type"`
}

func (c *Client) MovieDetail(ctx context.Context, id int) (*MovieDetail, error) {
	q := url.Values{}
	q.Set("append_to_response", "videos,external_ids")

	var out MovieDetail
	if err := c.get(ctx, fmt.Sprintf("/movie/%d", id), q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (d *MovieDetail) TrailerKey() string {
	for _, v := range d.Videos.Results {
		if v.Site == "YouTube" && v.Type == "Trailer" {
			return v.Key
		}
	}
	return ""
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	u := c.baseURL + path
	if q != nil {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tmdb: %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func joinInts(ints []int) string {
	s := make([]string, len(ints))
	for i, v := range ints {
		s[i] = strconv.Itoa(v)
	}
	return strings.Join(s, ",")
}
