package botmgr

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
)

var trace = os.Getenv("TGBOTD_TRACE") == "1"
var traceFile *os.File
var traceMu sync.Mutex

func init() {
	if trace {
		path := os.Getenv("TGBOTD_TRACE_FILE")
		if path == "" {
			path = "tgbotd_trace.log"
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			traceFile = f
			fmt.Fprintf(f, "\n=== tgbotd started at %s ===\n", time.Now().Format(time.RFC3339))
			f.Sync()
		} else {
			fmt.Fprintf(os.Stderr, "trace file open failed: %v\n", err)
		}
	}
}

func tracef(format string, args ...any) {
	if !trace || traceFile == nil {
		return
	}
	traceMu.Lock()
	fmt.Fprintf(traceFile, "[%s][buf] "+format+"\n", append([]any{time.Now().Format("15:04:05.000")}, args...)...)
	traceFile.Sync()
	traceMu.Unlock()
}

func Tracef(format string, args ...any) { tracef(format, args...) }

// updateItem is a queued update with its permanently assigned Bot API update_id.
type updateItem struct {
	ID     int64
	Update telegram.Update
}

// UpdateBuffer is a per-bot bounded queue of MTProto updates. IDs are
// assigned at Push time (never re-assigned) so retries + offset semantics
// work correctly. Access is serialized by getUpdates via TakeLock.
type UpdateBuffer struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []updateItem
	nextID int64
	cap    int

	// takeMu serializes concurrent long-poll drains (getUpdates OR webhook).
	takeMu sync.Mutex
}

const defaultBufferCap = 10_000

func NewUpdateBuffer() *UpdateBuffer {
	b := &UpdateBuffer{nextID: 1, cap: defaultBufferCap}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Push assigns the next monotonic update_id and appends. If the buffer is
// full, the oldest item is dropped (matching Bot API's "at most N pending
// updates" behavior — clients that fall behind lose the tail, not the head
// of new updates).
func (b *UpdateBuffer) Push(u telegram.Update) {
	b.mu.Lock()
	if len(b.items) >= b.cap {
		copy(b.items, b.items[1:])
		b.items = b.items[:len(b.items)-1]
	}
	id := b.nextID
	b.nextID++
	b.items = append(b.items, updateItem{ID: id, Update: u})
	tracef("Push id=%d type=%s buf_len=%d", id, reflect.TypeOf(u), len(b.items))
	b.cond.Broadcast()
	b.mu.Unlock()
}

// TakeLock returns a function that must be called to release the drain lock.
// Only one goroutine at a time may drain the buffer.
func (b *UpdateBuffer) TakeLock() func() {
	b.takeMu.Lock()
	return b.takeMu.Unlock
}

// Peek returns a snapshot of up to `limit` items whose IDs are >= offset,
// without removing them. Caller must hold TakeLock.
func (b *UpdateBuffer) Peek(offset int64, limit int) []updateItem {
	b.mu.Lock()
	defer b.mu.Unlock()
	if offset > 0 {
		i := 0
		for i < len(b.items) && b.items[i].ID < offset {
			i++
		}
		if i > 0 {
			b.items = append(b.items[:0], b.items[i:]...)
		}
	}
	n := len(b.items)
	if limit > 0 && n > limit {
		n = limit
	}
	out := make([]updateItem, n)
	copy(out, b.items[:n])
	return out
}

// Wait blocks up to `timeout` for at least one item, or ctx cancel. Returns
// snapshot of current items (Peek semantics with offset=0). Caller must hold
// TakeLock.
func (b *UpdateBuffer) Wait(ctx context.Context, offset int64, limit int, timeout time.Duration) []updateItem {
	b.mu.Lock()
	if hasReady(b.items, offset) {
		snap := snapshotLocked(b.items, offset, limit)
		b.mu.Unlock()
		return snap
	}
	if timeout <= 0 {
		b.mu.Unlock()
		return nil
	}

	done := make(chan struct{})
	defer close(done)
	if ctx != nil {
		go func() {
			select {
			case <-ctx.Done():
				b.cond.Broadcast()
			case <-done:
			}
		}()
	}
	timer := time.AfterFunc(timeout, func() { b.cond.Broadcast() })
	defer timer.Stop()

	deadline := time.Now().Add(timeout)
	for !hasReady(b.items, offset) && time.Now().Before(deadline) {
		if ctx != nil && ctx.Err() != nil {
			break
		}
		b.cond.Wait()
	}
	snap := snapshotLocked(b.items, offset, limit)
	b.mu.Unlock()
	return snap
}

func hasReady(items []updateItem, offset int64) bool {
	for _, it := range items {
		if it.ID >= offset {
			return true
		}
	}
	return false
}

func snapshotLocked(items []updateItem, offset int64, limit int) []updateItem {
	if offset > 0 {
		i := 0
		for i < len(items) && items[i].ID < offset {
			i++
		}
		items = items[i:]
	}
	n := len(items)
	if limit > 0 && n > limit {
		n = limit
	}
	out := make([]updateItem, n)
	copy(out, items[:n])
	return out
}

// Ack drops all items with ID < upTo. Caller must hold TakeLock.
func (b *UpdateBuffer) Ack(upTo int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := 0
	for i < len(b.items) && b.items[i].ID < upTo {
		i++
	}
	if i > 0 {
		tracef("Ack upTo=%d dropped=%d remaining=%d", upTo, i, len(b.items)-i)
		b.items = append(b.items[:0], b.items[i:]...)
	}
}

// Clear removes everything.
func (b *UpdateBuffer) Clear() {
	b.mu.Lock()
	b.items = b.items[:0]
	b.mu.Unlock()
}

func (b *UpdateBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.items)
}
