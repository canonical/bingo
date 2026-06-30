package paste_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"bingo/internal/paste"
)

// mockRepository implements paste.Repository for testing.
type mockRepository struct {
	mu              sync.Mutex
	deleteExpiredCalls int
}

func (m *mockRepository) Create(ctx context.Context, params paste.CreateParams) (*paste.Paste, error) {
	return nil, nil
}

func (m *mockRepository) GetByKey(ctx context.Context, key string) (*paste.Paste, error) {
	return nil, nil
}

func (m *mockRepository) Delete(ctx context.Context, key string) error {
	return nil
}

func (m *mockRepository) DeleteExpired(ctx context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteExpiredCalls++
	return 1, nil
}

func TestStartSweep_callsDeleteExpired(t *testing.T) {
	mock := &mockRepository{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start sweep with very short interval (5ms)
	cancelSweep := paste.StartSweep(ctx, mock, 5*time.Millisecond)

	// Wait for at least one call to happen
	time.Sleep(20 * time.Millisecond)

	// Verify DeleteExpired was called at least once
	mock.mu.Lock()
	calls := mock.deleteExpiredCalls
	mock.mu.Unlock()

	if calls < 1 {
		t.Errorf("DeleteExpired called %d times, want >= 1", calls)
	}

	// Clean up — cancel blocks until goroutine exits via wg.Wait().
	cancelSweep()
}

func TestStartSweep_cancel(t *testing.T) {
	mock := &mockRepository{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start sweep
	cancelSweep := paste.StartSweep(ctx, mock, 5*time.Millisecond)

	// Let it run a bit
	time.Sleep(15 * time.Millisecond)

	// Get initial call count
	mock.mu.Lock()
	initialCalls := mock.deleteExpiredCalls
	mock.mu.Unlock()

	// Cancel blocks until goroutine exits via wg.Wait(); finalCalls is stable immediately.
	cancelSweep()

	// Get final call count — allow for one tick that raced before cancel.
	mock.mu.Lock()
	finalCalls := mock.deleteExpiredCalls
	mock.mu.Unlock()

	if finalCalls > initialCalls+1 {
		t.Errorf("after cancel, DeleteExpired continued being called (initial: %d, final: %d)", initialCalls, finalCalls)
	}
}
