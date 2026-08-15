package store

import (
	"context"
	"testing"
)

func TestSetRoomFiltersClearsQueue(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	room, err := s.CreateRoom(ctx, "test", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	member, err := s.JoinRoom(ctx, room.ID, "Alice")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.AppendQueue(ctx, room.ID, []int64{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceRoomCursor(ctx, room.ID, 5, true); err != nil {
		t.Fatal(err)
	}

	count, err := s.CountUnswipedQueue(ctx, room.ID, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("count before filter change = %d, want 3", count)
	}

	if err := s.SetRoomFilters(ctx, room.ID, `{"min_rating":7}`); err != nil {
		t.Fatal(err)
	}

	count, err = s.CountUnswipedQueue(ctx, room.ID, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("count after filter change = %d, want 0 (deck cleared)", count)
	}

	got, err := s.RoomByCode(ctx, room.Code)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filters != `{"min_rating":7}` {
		t.Fatalf("filters = %q, want the new value", got.Filters)
	}
	if got.PageCursor != 0 || got.Exhausted {
		t.Fatalf("expected cursor reset, got page_cursor=%d exhausted=%v", got.PageCursor, got.Exhausted)
	}
}
