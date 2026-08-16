package store

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndJoinRoom(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	room, err := s.CreateRoom(ctx, "movie night", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(room.Code) != 6 {
		t.Fatalf("code = %q, want length 6", room.Code)
	}

	got, err := s.RoomByCode(ctx, room.Code)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != room.ID {
		t.Fatalf("RoomByCode returned room %d, want %d", got.ID, room.ID)
	}

	m1, err := s.JoinRoom(ctx, room.ID, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if m1.Token == "" {
		t.Fatal("expected non-empty token")
	}

	m1Again, err := s.JoinRoom(ctx, room.ID, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if m1Again.ID != m1.ID || m1Again.Token != m1.Token {
		t.Fatalf("rejoining with same name should reuse member, got different: %+v vs %+v", m1, m1Again)
	}

	m2, err := s.JoinRoom(ctx, room.ID, "Bob")
	if err != nil {
		t.Fatal(err)
	}
	if m2.ID == m1.ID {
		t.Fatal("expected a distinct member for Bob")
	}

	members, err := s.MembersByRoom(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatalf("len(members) = %d, want 2", len(members))
	}

	byToken, err := s.MemberByToken(ctx, m2.Token)
	if err != nil {
		t.Fatal(err)
	}
	if byToken.Name != "Bob" {
		t.Fatalf("MemberByToken name = %q, want Bob", byToken.Name)
	}
}

func TestJoinRoomDedupesNameCaseAndWhitespace(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	room, err := s.CreateRoom(ctx, "movie night", `{}`)
	if err != nil {
		t.Fatal(err)
	}

	m1, err := s.JoinRoom(ctx, room.ID, "Jenny")
	if err != nil {
		t.Fatal(err)
	}
	m2, err := s.JoinRoom(ctx, room.ID, " jenny ")
	if err != nil {
		t.Fatal(err)
	}
	if m2.ID != m1.ID || m2.Token != m1.Token {
		t.Fatalf("rejoining with different case/whitespace should reuse member, got different: %+v vs %+v", m1, m2)
	}

	members, err := s.MembersByRoom(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 {
		t.Fatalf("len(members) = %d, want 1 (no duplicate ghost member)", len(members))
	}
}

func TestDeleteMemberUnblocksMatches(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	room, err := s.CreateRoom(ctx, "test", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	alice, err := s.JoinRoom(ctx, room.ID, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := s.JoinRoom(ctx, room.ID, "Bob")
	if err != nil {
		t.Fatal(err)
	}
	ghost, err := s.JoinRoom(ctx, room.ID, "Ghost")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertMovieBasic(ctx, Movie{TmdbID: 1, Title: "Movie"}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.RecordSwipe(ctx, room.ID, alice.ID, 1, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordSwipe(ctx, room.ID, bob.ID, 1, true); err != nil {
		t.Fatal(err)
	}

	matches, err := s.Matches(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches = %+v, want none while ghost member hasn't liked", matches)
	}

	if err := s.DeleteMember(ctx, ghost.ID); err != nil {
		t.Fatal(err)
	}

	matches, err = s.Matches(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].TmdbID != 1 {
		t.Fatalf("matches = %+v, want movie 1 once ghost member removed", matches)
	}
}

func TestRoomByCodeNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.RoomByCode(context.Background(), "ZZZZZZ")
	if err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
