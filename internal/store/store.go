package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

type Room struct {
	ID         int64
	Code       string
	Name       string
	Filters    string
	PageCursor int
	Exhausted  bool
	CreatedAt  int64
}

type Member struct {
	ID            int64
	RoomID        int64
	Token         string
	Name          string
	MatchesSeenAt int64
	CreatedAt     int64
}

const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func randomString(alphabet string, n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}

func (s *Store) CreateRoom(ctx context.Context, name, filters string) (*Room, error) {
	for attempt := 0; attempt < 5; attempt++ {
		code, err := randomString(codeAlphabet, 6)
		if err != nil {
			return nil, err
		}

		now := time.Now().Unix()
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO rooms (code, name, filters, created_at) VALUES (?, ?, ?, ?)`,
			code, name, filters, now)
		if err != nil {
			if isUniqueConstraint(err) {
				continue
			}
			return nil, err
		}

		id, err := res.LastInsertId()
		if err != nil {
			return nil, err
		}
		return &Room{ID: id, Code: code, Name: name, Filters: filters, CreatedAt: now}, nil
	}
	return nil, errors.New("could not generate a unique room code")
}

func (s *Store) RoomByCode(ctx context.Context, code string) (*Room, error) {
	var r Room
	var exhausted int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, code, name, filters, page_cursor, exhausted, created_at FROM rooms WHERE code = ?`,
		code,
	).Scan(&r.ID, &r.Code, &r.Name, &r.Filters, &r.PageCursor, &exhausted, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.Exhausted = exhausted != 0
	return &r, nil
}

func (s *Store) SetRoomFilters(ctx context.Context, roomID int64, filters string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE rooms SET filters = ?, page_cursor = 0, exhausted = 0 WHERE id = ?`,
		filters, roomID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM queue WHERE room_id = ?`, roomID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AdvanceRoomCursor(ctx context.Context, roomID int64, cursor int, exhausted bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE rooms SET page_cursor = ?, exhausted = ? WHERE id = ?`,
		cursor, exhausted, roomID)
	return err
}

func (s *Store) JoinRoom(ctx context.Context, roomID int64, name string) (*Member, error) {
	existing, err := s.memberByRoomAndName(ctx, roomID, name)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	token, err := randomString("abcdefghijklmnopqrstuvwxyz0123456789", 32)
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO members (room_id, token, name, created_at) VALUES (?, ?, ?, ?)`,
		roomID, token, name, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Member{ID: id, RoomID: roomID, Token: token, Name: name, CreatedAt: now}, nil
}

func (s *Store) memberByRoomAndName(ctx context.Context, roomID int64, name string) (*Member, error) {
	var m Member
	err := s.db.QueryRowContext(ctx,
		`SELECT id, room_id, token, name, matches_seen_at, created_at FROM members WHERE room_id = ? AND name = ?`,
		roomID, name,
	).Scan(&m.ID, &m.RoomID, &m.Token, &m.Name, &m.MatchesSeenAt, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) MemberByToken(ctx context.Context, token string) (*Member, error) {
	var m Member
	err := s.db.QueryRowContext(ctx,
		`SELECT id, room_id, token, name, matches_seen_at, created_at FROM members WHERE token = ?`,
		token,
	).Scan(&m.ID, &m.RoomID, &m.Token, &m.Name, &m.MatchesSeenAt, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) MembersByRoom(ctx context.Context, roomID int64) ([]Member, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, room_id, token, name, matches_seen_at, created_at FROM members WHERE room_id = ? ORDER BY created_at`,
		roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.RoomID, &m.Token, &m.Name, &m.MatchesSeenAt, &m.CreatedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *Store) StampMatchesSeen(ctx context.Context, memberID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE members SET matches_seen_at = ? WHERE id = ?`,
		time.Now().Unix(), memberID)
	return err
}

type PushSubscription struct {
	MemberID   int64
	MemberName string
	Endpoint   string
	P256dh     string
	Auth       string
}

func (s *Store) SavePushSubscription(ctx context.Context, memberID int64, endpoint, p256dh, auth string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO push_subscriptions (member_id, endpoint, p256dh, auth, created_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(endpoint) DO UPDATE SET member_id = excluded.member_id, p256dh = excluded.p256dh, auth = excluded.auth`,
		memberID, endpoint, p256dh, auth, time.Now().Unix())
	return err
}

func (s *Store) DeletePushSubscription(ctx context.Context, memberID int64, endpoint string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM push_subscriptions WHERE member_id = ? AND endpoint = ?`, memberID, endpoint)
	return err
}

func (s *Store) DeletePushSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	return err
}

// PushSubscriptionsForRoomExcept returns push subscriptions for every member
// of the room other than memberID, e.g. to notify roommates that the given
// member just completed a match.
func (s *Store) PushSubscriptionsForRoomExcept(ctx context.Context, roomID, memberID int64) ([]PushSubscription, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ps.member_id, m.name, ps.endpoint, ps.p256dh, ps.auth
		FROM push_subscriptions ps
		JOIN members m ON m.id = ps.member_id
		WHERE m.room_id = ? AND ps.member_id != ?`,
		roomID, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []PushSubscription
	for rows.Next() {
		var sub PushSubscription
		if err := rows.Scan(&sub.MemberID, &sub.MemberName, &sub.Endpoint, &sub.P256dh, &sub.Auth); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

type Movie struct {
	TmdbID     int64
	Title      string
	Year       int
	Overview   string
	PosterPath string
	Runtime    int
	Genres     string
	ImdbID     string
	TrailerKey string
	TmdbRating float64
	TmdbVotes  int
	ImdbRating string
	RtRating   string
	DetailAt   int64
	RatingsAt  int64
}

func (s *Store) UpsertMovieBasic(ctx context.Context, m Movie) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO movies (tmdb_id, title, year, overview, poster_path, genres, tmdb_rating, tmdb_votes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tmdb_id) DO UPDATE SET
			title = excluded.title, year = excluded.year, overview = excluded.overview,
			poster_path = excluded.poster_path, genres = excluded.genres,
			tmdb_rating = excluded.tmdb_rating, tmdb_votes = excluded.tmdb_votes`,
		m.TmdbID, m.Title, m.Year, m.Overview, m.PosterPath, m.Genres, m.TmdbRating, m.TmdbVotes)
	return err
}

func (s *Store) MovieByID(ctx context.Context, tmdbID int64) (*Movie, error) {
	m, err := scanMovie(s.db.QueryRowContext(ctx, movieSelect+` WHERE tmdb_id = ?`, tmdbID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

func (s *Store) SetMovieDetail(ctx context.Context, tmdbID int64, runtime int, imdbID, trailerKey string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE movies SET runtime = ?, imdb_id = ?, trailer_key = ?, detail_at = ? WHERE tmdb_id = ?`,
		runtime, imdbID, trailerKey, time.Now().Unix(), tmdbID)
	return err
}

func (s *Store) SetMovieRatings(ctx context.Context, tmdbID int64, imdbRating, rtRating string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE movies SET imdb_rating = ?, rt_rating = ?, ratings_at = ? WHERE tmdb_id = ?`,
		imdbRating, rtRating, time.Now().Unix(), tmdbID)
	return err
}

func (s *Store) AppendQueue(ctx context.Context, roomID int64, tmdbIDs []int64) (added int, err error) {
	if len(tmdbIDs) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var maxPos sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(position) FROM queue WHERE room_id = ?`, roomID).Scan(&maxPos); err != nil {
		return 0, err
	}
	pos := int(maxPos.Int64)

	for _, id := range tmdbIDs {
		pos++
		res, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO queue (room_id, position, tmdb_id) VALUES (?, ?, ?)`,
			roomID, pos, id)
		if err != nil {
			return 0, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			added++
		} else {
			pos--
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return added, nil
}

func (s *Store) CountUnswipedQueue(ctx context.Context, roomID, memberID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM queue q
		WHERE q.room_id = ?
		AND NOT EXISTS (SELECT 1 FROM swipes s WHERE s.member_id = ? AND s.tmdb_id = q.tmdb_id)`,
		roomID, memberID).Scan(&n)
	return n, err
}

func (s *Store) CardsForMember(ctx context.Context, roomID, memberID int64, limit int) ([]Movie, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+movieColumns+`
		FROM queue q
		JOIN movies m ON m.tmdb_id = q.tmdb_id
		WHERE q.room_id = ?
		AND NOT EXISTS (SELECT 1 FROM swipes s WHERE s.member_id = ? AND s.tmdb_id = q.tmdb_id)
		ORDER BY q.position
		LIMIT ?`,
		roomID, memberID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movies []Movie
	for rows.Next() {
		m, err := scanMovieRow(rows)
		if err != nil {
			return nil, err
		}
		movies = append(movies, *m)
	}
	return movies, rows.Err()
}

// RecordSwipe records a member's swipe and reports whether it just completed
// a new match, i.e. this is the swipe that made every member in the room
// like the movie.
func (s *Store) RecordSwipe(ctx context.Context, roomID, memberID, tmdbID int64, liked bool) (bool, error) {
	var wasLiked bool
	err := s.db.QueryRowContext(ctx,
		`SELECT liked FROM swipes WHERE member_id = ? AND tmdb_id = ?`, memberID, tmdbID,
	).Scan(&wasLiked)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO swipes (member_id, room_id, tmdb_id, liked, created_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(member_id, tmdb_id) DO UPDATE SET liked = excluded.liked, created_at = excluded.created_at`,
		memberID, roomID, tmdbID, liked, time.Now().Unix(),
	); err != nil {
		return false, err
	}

	if !liked || wasLiked {
		return false, nil
	}

	var memberCount, likeCount int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM members WHERE room_id = ?`, roomID,
	).Scan(&memberCount); err != nil {
		return false, err
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM swipes WHERE room_id = ? AND tmdb_id = ? AND liked = 1`, roomID, tmdbID,
	).Scan(&likeCount); err != nil {
		return false, err
	}
	return memberCount > 1 && likeCount == memberCount, nil
}

func (s *Store) DeleteSwipe(ctx context.Context, memberID, tmdbID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM swipes WHERE member_id = ? AND tmdb_id = ?`, memberID, tmdbID)
	return err
}

func (s *Store) CountSwipes(ctx context.Context, memberID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM swipes WHERE member_id = ?`, memberID).Scan(&n)
	return n, err
}

func (s *Store) LikesByMember(ctx context.Context, memberID int64) ([]Movie, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+movieColumns+`
		FROM movies m
		JOIN swipes s ON s.tmdb_id = m.tmdb_id
		WHERE s.member_id = ? AND s.liked = 1
		ORDER BY s.created_at DESC`,
		memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movies []Movie
	for rows.Next() {
		m, err := scanMovieRow(rows)
		if err != nil {
			return nil, err
		}
		movies = append(movies, *m)
	}
	return movies, rows.Err()
}

const matchesWhere = `
	FROM movies m
	JOIN swipes s ON s.tmdb_id = m.tmdb_id AND s.room_id = ? AND s.liked = 1
	GROUP BY m.tmdb_id
	HAVING COUNT(DISTINCT s.member_id) = (SELECT COUNT(*) FROM members WHERE room_id = ?)
	   AND (SELECT COUNT(*) FROM members WHERE room_id = ?) > 1`

func (s *Store) Matches(ctx context.Context, roomID int64) ([]Movie, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+movieColumns+matchesWhere+` ORDER BY MAX(s.created_at) DESC`,
		roomID, roomID, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movies []Movie
	for rows.Next() {
		m, err := scanMovieRow(rows)
		if err != nil {
			return nil, err
		}
		movies = append(movies, *m)
	}
	return movies, rows.Err()
}

func (s *Store) UnseenMatchCount(ctx context.Context, roomID int64, seenAt int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM (SELECT m.tmdb_id`+matchesWhere+` AND MAX(s.created_at) > ?)`,
		roomID, roomID, roomID, seenAt).Scan(&n)
	return n, err
}

const movieColumns = `m.tmdb_id, m.title, COALESCE(m.year,0), COALESCE(m.overview,''), COALESCE(m.poster_path,''),
	COALESCE(m.runtime,0), COALESCE(m.genres,''), COALESCE(m.imdb_id,''), COALESCE(m.trailer_key,''),
	COALESCE(m.tmdb_rating,0), COALESCE(m.tmdb_votes,0), COALESCE(m.imdb_rating,''), COALESCE(m.rt_rating,''),
	m.detail_at, m.ratings_at`

const movieSelect = `SELECT ` + movieColumns + ` FROM movies m`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanMovie(row rowScanner) (*Movie, error) {
	return scanMovieRow(row)
}

func scanMovieRow(row rowScanner) (*Movie, error) {
	var m Movie
	err := row.Scan(&m.TmdbID, &m.Title, &m.Year, &m.Overview, &m.PosterPath,
		&m.Runtime, &m.Genres, &m.ImdbID, &m.TrailerKey,
		&m.TmdbRating, &m.TmdbVotes, &m.ImdbRating, &m.RtRating,
		&m.DetailAt, &m.RatingsAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
