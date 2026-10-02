package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/brk3/movieswipe/internal/omdb"
	"github.com/brk3/movieswipe/internal/store"
	"github.com/brk3/movieswipe/internal/tmdb"
)

const maxTmdbPage = 500
const enrichWorkers = 5
const defaultMinVotes = 100
const defaultMinRating = 5.0

type Filters struct {
	YearFrom  int     `json:"year_from,omitempty"`
	YearTo    int     `json:"year_to,omitempty"`
	GenreIDs  []int   `json:"genre_ids,omitempty"`
	MinRating float64 `json:"min_rating,omitempty"`
	MinVotes  int     `json:"min_votes,omitempty"`
}

type Catalog struct {
	tmdb  *tmdb.Client
	omdb  *omdb.Client
	store *store.Store
}

func New(tmdbClient *tmdb.Client, omdbClient *omdb.Client, st *store.Store) *Catalog {
	return &Catalog{tmdb: tmdbClient, omdb: omdbClient, store: st}
}

func (c *Catalog) EnsureCards(ctx context.Context, room *store.Room, memberID int64, want int) error {
	unswiped, err := c.store.CountUnswipedQueue(ctx, room.ID, memberID)
	if err != nil {
		return err
	}

	var filters Filters
	if room.Filters != "" {
		if err := json.Unmarshal([]byte(room.Filters), &filters); err != nil {
			return fmt.Errorf("parse filters: %w", err)
		}
	}

	cursor := room.PageCursor
	exhausted := room.Exhausted

	for unswiped < want && !exhausted {
		if c.tmdb == nil {
			break
		}
		page := cursor + 1
		if page > maxTmdbPage {
			exhausted = true
			break
		}

		minVotes := filters.MinVotes
		if minVotes <= 0 {
			minVotes = defaultMinVotes
		}
		minRating := filters.MinRating
		if minRating <= 0 {
			minRating = defaultMinRating
		}

		results, totalPages, err := c.tmdb.Discover(ctx, tmdb.DiscoverFilters{
			Page:      page,
			YearFrom:  filters.YearFrom,
			YearTo:    filters.YearTo,
			GenreIDs:  filters.GenreIDs,
			MinRating: minRating,
			MinVotes:  minVotes,
		})
		if err != nil {
			return err
		}
		cursor = page
		if len(results) == 0 || page >= totalPages {
			exhausted = true
		}

		ids := make([]int64, 0, len(results))
		for _, res := range results {
			if err := c.store.UpsertMovieBasic(ctx, basicMovieFromDiscover(res)); err != nil {
				return err
			}
			ids = append(ids, int64(res.ID))
		}

		added, err := c.store.AppendQueue(ctx, room.ID, ids)
		if err != nil {
			return err
		}
		unswiped += added
	}

	if cursor != room.PageCursor || exhausted != room.Exhausted {
		if err := c.store.AdvanceRoomCursor(ctx, room.ID, cursor, exhausted); err != nil {
			return err
		}
		room.PageCursor = cursor
		room.Exhausted = exhausted
	}
	return nil
}

func (c *Catalog) EnrichCards(ctx context.Context, movies []store.Movie) []store.Movie {
	jobs := make(chan int)
	var wg sync.WaitGroup

	wg.Add(enrichWorkers)
	for i := 0; i < enrichWorkers; i++ {
		go func() {
			defer wg.Done()
			for idx := range jobs {
				c.enrichOne(ctx, &movies[idx])
			}
		}()
	}
	for i := range movies {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return movies
}

func (c *Catalog) enrichOne(ctx context.Context, m *store.Movie) {
	if m.DetailAt == 0 && c.tmdb != nil {
		detail, err := c.tmdb.MovieDetail(ctx, int(m.TmdbID))
		if err == nil {
			trailerKey := detail.TrailerKey()
			if err := c.store.SetMovieDetail(ctx, m.TmdbID, detail.Runtime, detail.ExternalIDs.IMDbID, trailerKey); err == nil {
				m.Runtime = detail.Runtime
				m.ImdbID = detail.ExternalIDs.IMDbID
				m.TrailerKey = trailerKey
				m.DetailAt = time.Now().Unix()
			}
		}
	}

	if m.RatingsAt == 0 && m.ImdbID != "" && c.omdb != nil {
		ratings, err := c.omdb.Ratings(ctx, m.ImdbID)
		if err == nil {
			if err := c.store.SetMovieRatings(ctx, m.TmdbID, ratings.IMDb, ratings.RottenTomatoes); err == nil {
				m.ImdbRating = ratings.IMDb
				m.RtRating = ratings.RottenTomatoes
				m.RatingsAt = time.Now().Unix()
			}
		}
	}
}

func basicMovieFromDiscover(r tmdb.DiscoverResult) store.Movie {
	year := 0
	if len(r.ReleaseDate) >= 4 {
		fmt.Sscanf(r.ReleaseDate[:4], "%d", &year)
	}
	genresJSON, _ := json.Marshal(r.GenreIDs)
	return store.Movie{
		TmdbID:     int64(r.ID),
		Title:      r.Title,
		Year:       year,
		Overview:   r.Overview,
		PosterPath: r.PosterPath,
		Genres:     string(genresJSON),
		TmdbRating: r.VoteAverage,
		TmdbVotes:  r.VoteCount,
	}
}
