package tcheck

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// Helper to wait for goroutine completion in tests
func runCheckItemAsync(ci *CheckItem) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ci.Run()
	}()
	wg.Wait()
}

func TestNewCheckItem_InitialState(t *testing.T) {
	fn := func(SubProgressReporter) error { return nil }
	item := NewCheckItem(1, "test", fn)

	if item.ID != 1 {
		t.Errorf("expected ID 1, got %d", item.ID)
	}
	if item.Name != "test" {
		t.Errorf("expected Name 'test', got %s", item.Name)
	}
	if item.Status != StatusPending {
		t.Errorf("expected StatusPending, got %v", item.Status)
	}
	if item.runFunc == nil {
		t.Error("expected runFunc to be set")
	}
}

func TestCheckItem_Run_Success(t *testing.T) {
	progressUpdates := []struct {
		percentage int
		message    string
	}{
		{10, "Started"},
		{50, "Halfway"},
		{100, "Done"},
	}
	fn := func(r SubProgressReporter) error {
		for _, u := range progressUpdates {
			r.ReportSubProgress(u.percentage, u.message)
		}
		return nil
	}
	item := NewCheckItem(2, "success", fn)
	item.Run()

	if item.Status != StatusCompleted {
		t.Errorf("expected StatusCompleted, got %v", item.Status)
	}
	if item.SubProgress != 100 {
		t.Errorf("expected SubProgress 100, got %d", item.SubProgress)
	}
	if item.Error != nil {
		t.Errorf("expected no error, got %v", item.Error)
	}
}

func TestCheckItem_Run_Failure(t *testing.T) {
	expectedErr := errors.New("fail")
	fn := func(r SubProgressReporter) error {
		r.ReportSubProgress(30, "Failing soon")
		return expectedErr
	}
	item := NewCheckItem(3, "fail", fn)
	item.Run()

	if item.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %v", item.Status)
	}
	if item.Error != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, item.Error)
	}
}

func TestCheckItem_SubProgressBounds(t *testing.T) {
	fn := func(r SubProgressReporter) error {
		r.ReportSubProgress(-10, "Too low")
		r.ReportSubProgress(110, "Too high")
		return nil
	}
	item := NewCheckItem(4, "bounds", fn)
	item.Run()

	if item.SubProgress != 100 {
		t.Errorf("expected SubProgress 100, got %d", item.SubProgress)
	}
}

func TestCheckItem_ConcurrentProgress(t *testing.T) {
	fn := func(r SubProgressReporter) error {
		var wg sync.WaitGroup
		for i := 0; i <= 100; i += 10 {
			wg.Add(1)
			go func(p int) {
				defer wg.Done()
				r.ReportSubProgress(p, "")
			}(i)
		}
		wg.Wait()
		return nil
	}
	item := NewCheckItem(5, "concurrent", fn)
	item.Run()

	if item.Status != StatusCompleted {
		t.Errorf("expected StatusCompleted, got %v", item.Status)
	}
}

func TestCheckItem_SubMessage(t *testing.T) {
	fn := func(r SubProgressReporter) error {
		r.ReportSubProgress(42, "The answer")
		return nil
	}
	item := NewCheckItem(6, "message", fn)
	item.Run()

	if item.SubMessage != "The answer" {
		t.Errorf("expected SubMessage 'The answer', got %q", item.SubMessage)
	}
}

func TestCheckItem_StatusTransitions(t *testing.T) {
	fn := func(r SubProgressReporter) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}
	item := NewCheckItem(7, "status", fn)

	if item.Status != StatusPending {
		t.Errorf("expected StatusPending before run, got %v", item.Status)
	}

	go item.Run()
	time.Sleep(1 * time.Millisecond) // Let goroutine start

	item.mu.Lock()
	status := item.Status
	item.mu.Unlock()
	if status != StatusInProgress {
		t.Errorf("expected StatusInProgress during run, got %v", status)
	}

	time.Sleep(20 * time.Millisecond) // Wait for completion

	if item.Status != StatusCompleted {
		t.Errorf("expected StatusCompleted after run, got %v", item.Status)
	}
}