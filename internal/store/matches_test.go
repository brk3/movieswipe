package store

import (
	"context"
	"testing"
)

func TestMatches(t *testing.T) {
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

	for _, id := range []int64{1, 2, 3} {
		if err := s.UpsertMovieBasic(ctx, Movie{TmdbID: id, Title: "Movie"}); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.RecordSwipe(ctx, room.ID, alice.ID, 1, true); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordSwipe(ctx, room.ID, bob.ID, 1, true); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordSwipe(ctx, room.ID, alice.ID, 2, true); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordSwipe(ctx, room.ID, bob.ID, 2, false); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordSwipe(ctx, room.ID, alice.ID, 3, true); err != nil {
		t.Fatal(err)
	}

	matches, err := s.Matches(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].TmdbID != 1 {
		t.Fatalf("matches = %+v, want just movie 1", matches)
	}

	unseen, err := s.UnseenMatchCount(ctx, room.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if unseen != 1 {
		t.Fatalf("unseen = %d, want 1", unseen)
	}

	aliceLikes, err := s.LikesByMember(ctx, alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(aliceLikes) != 3 {
		t.Fatalf("len(aliceLikes) = %d, want 3", len(aliceLikes))
	}
}

func TestMatchesSoloRoomYieldsNothing(t *testing.T) {
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
	if err := s.UpsertMovieBasic(ctx, Movie{TmdbID: 1, Title: "Movie"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordSwipe(ctx, room.ID, alice.ID, 1, true); err != nil {
		t.Fatal(err)
	}

	matches, err := s.Matches(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches = %+v, want none for a solo room", matches)
	}
}
