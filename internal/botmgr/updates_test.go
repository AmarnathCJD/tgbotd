package botmgr

import (
	"context"
	"testing"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
)

func TestUpdateBufferSequentialFIFO(t *testing.T) {
	b := NewUpdateBuffer()

	// Push 5 updates
	for i := 0; i < 5; i++ {
		b.Push(&telegram.UpdateNewMessage{})
	}

	release := b.TakeLock()
	items := b.Wait(context.Background(), 0, 100, 100*time.Millisecond)
	release()

	if len(items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(items))
	}
	for i, it := range items {
		if it.ID != int64(i+1) {
			t.Errorf("item %d: expected id %d, got %d", i, i+1, it.ID)
		}
	}
}

func TestUpdateBufferOffsetSemantics(t *testing.T) {
	b := NewUpdateBuffer()

	// Push updates 1..5
	for i := 0; i < 5; i++ {
		b.Push(&telegram.UpdateNewMessage{})
	}

	// First poll: offset=0 → gets 1..5
	release := b.TakeLock()
	items := b.Wait(context.Background(), 0, 100, 10*time.Millisecond)
	release()
	if len(items) != 5 {
		t.Fatalf("first poll: expected 5, got %d", len(items))
	}

	// Client sends back offset=6 (i.e., last update_id + 1)
	release = b.TakeLock()
	b.Ack(6)
	release()

	// Buffer should be empty now
	if b.Len() != 0 {
		t.Fatalf("after ack: expected 0, got %d", b.Len())
	}

	// Push 3 more
	for i := 0; i < 3; i++ {
		b.Push(&telegram.UpdateNewMessage{})
	}

	release = b.TakeLock()
	items = b.Wait(context.Background(), 6, 100, 10*time.Millisecond)
	release()
	if len(items) != 3 {
		t.Fatalf("second batch: expected 3, got %d", len(items))
	}
	// IDs should be 6, 7, 8
	for i, it := range items {
		if it.ID != int64(i+6) {
			t.Errorf("item %d: expected id %d, got %d", i, i+6, it.ID)
		}
	}
}

func TestUpdateBufferBackToBackPollLikeBotClient(t *testing.T) {
	b := NewUpdateBuffer()

	// Simulate the pattern: user sends msg1, bot polls & acks, user sends msg2, bot polls & acks.
	var offset int64 = 0

	// User event 1
	b.Push(&telegram.UpdateNewMessage{})

	// Bot polls
	release := b.TakeLock()
	items := b.Wait(context.Background(), offset, 100, 10*time.Millisecond)
	release()
	if len(items) != 1 {
		t.Fatalf("first poll: expected 1, got %d", len(items))
	}
	offset = items[len(items)-1].ID + 1
	release = b.TakeLock()
	b.Ack(offset)
	release()

	// User event 2
	b.Push(&telegram.UpdateNewMessage{})

	// Bot polls again
	release = b.TakeLock()
	items = b.Wait(context.Background(), offset, 100, 10*time.Millisecond)
	release()
	if len(items) != 1 {
		t.Fatalf("second poll: expected 1, got %d; buffer len=%d, offset=%d", len(items), b.Len(), offset)
	}
	if items[0].ID != 2 {
		t.Fatalf("second poll: expected id 2, got %d", items[0].ID)
	}

	// User events 3 and 4 back-to-back
	b.Push(&telegram.UpdateNewMessage{})
	b.Push(&telegram.UpdateNewMessage{})

	offset = items[len(items)-1].ID + 1
	release = b.TakeLock()
	b.Ack(offset)
	items = b.Wait(context.Background(), offset, 100, 10*time.Millisecond)
	release()
	if len(items) != 2 {
		t.Fatalf("third poll: expected 2, got %d", len(items))
	}
}
