CREATE TABLE IF NOT EXISTS movies (
  tmdb_id INTEGER PRIMARY KEY,
  title TEXT NOT NULL, year INTEGER, overview TEXT,
  poster_path TEXT, runtime INTEGER, genres TEXT,
  imdb_id TEXT, trailer_key TEXT,
  tmdb_rating REAL, tmdb_votes INTEGER,
  imdb_rating TEXT, rt_rating TEXT,
  detail_at INTEGER NOT NULL DEFAULT 0,
  ratings_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS rooms (
  id INTEGER PRIMARY KEY,
  code TEXT UNIQUE NOT NULL,
  name TEXT,
  filters TEXT NOT NULL,
  page_cursor INTEGER NOT NULL DEFAULT 0,
  exhausted INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS members (
  id INTEGER PRIMARY KEY,
  room_id INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  token TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL,
  matches_seen_at INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS queue (
  room_id INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  tmdb_id INTEGER NOT NULL,
  PRIMARY KEY (room_id, position)
);
CREATE UNIQUE INDEX IF NOT EXISTS queue_room_movie ON queue(room_id, tmdb_id);

CREATE TABLE IF NOT EXISTS swipes (
  member_id INTEGER NOT NULL REFERENCES members(id) ON DELETE CASCADE,
  room_id INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  tmdb_id INTEGER NOT NULL,
  liked INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  PRIMARY KEY (member_id, tmdb_id)
);
