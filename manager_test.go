package tcheck

import (
	"sync"
	"testing"
	"time"
)

func TestNewCheckManager(t *testing.T) {
	updateFunc := func() {}

	cm := NewCheckManager(updateFunc, 3)

	if cm == nil {
		t.Fatal("NewCheckManager returned nil")
	}
	if len(cm.items) != 0 {
		t.Errorf("Expected empty items slice, got %d items", len(cm.items))
	}
	if cm.itemCounter != 0 {
		t.Errorf("Expected itemCounter to be 0, got %d", cm.itemCounter)
	}
	if cap(cm.activeWorkers) != 3 {
		t.Errorf("Expected activeWorkers capacity to be 3, got %d", cap(cm.activeWorkers))
	}
}

func TestNewCheckManagerMinWorkers(t *testing.T) {
	cm := NewCheckManager(nil, 0)
	if cap(cm.activeWorkers) != 1 {
		t.Errorf("Expected at least 1 worker when 0 specified, got %d", cap(cm.activeWorkers))
	}
}

func TestAddCheck(t *testing.T) {
	cm := NewCheckManager(nil, 1)

	cm.AddCheck("test check", testFunc)

	if len(cm.items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(cm.items))
	}
	if cm.itemCounter != 1 {
		t.Errorf("Expected itemCounter to be 1, got %d", cm.itemCounter)
	}
	if cm.items[0].Name != "test check" {
		t.Errorf("Expected item name 'test check', got '%s'", cm.items[0].Name)
	}
}

func TestGetItems(t *testing.T) {
	cm := NewCheckManager(nil, 1)

	cm.AddCheck("check1", testFunc)
	cm.AddCheck("check2", testFunc)

	items := cm.GetItems()

	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}

	// Verify it's a copy by modifying the returned slice
	items[0] = nil
	if cm.items[0] == nil {
		t.Error("GetItems did not return a copy, original slice was modified")
	}
}

func TestCalculateOverallProgress(t *testing.T) {
	cm := NewCheckManager(nil, 1)

	// Test empty manager
	completed, total, percentage := cm.CalculateOverallProgress()
	if completed != 0 || total != 0 || percentage != 0 {
		t.Errorf("Expected (0,0,0) for empty manager, got (%d,%d,%d)", completed, total, percentage)
	}

	// Add some checks
	cm.AddCheck("check1", testFunc)
	cm.AddCheck("check2", testFunc)
	cm.AddCheck("check3", testFunc)

	// Initially all pending
	completed, total, percentage = cm.CalculateOverallProgress()
	if completed != 0 || total != 3 || percentage != 0 {
		t.Errorf("Expected (0,3,0) for all pending, got (%d,%d,%d)", completed, total, percentage)
	}

	// Mark one as completed
	cm.items[0].mu.Lock()
	cm.items[0].Status = StatusCompleted
	cm.items[0].mu.Unlock()

	completed, total, percentage = cm.CalculateOverallProgress()
	if completed != 1 || total != 3 || percentage != 33 {
		t.Errorf("Expected (1,3,33) for one completed, got (%d,%d,%d)", completed, total, percentage)
	}

	// Mark one as failed
	cm.items[1].mu.Lock()
	cm.items[1].Status = StatusFailed
	cm.items[1].mu.Unlock()

	completed, total, percentage = cm.CalculateOverallProgress()
	if completed != 2 || total != 3 || percentage != 66 {
		t.Errorf("Expected (2,3,66) for one completed and one failed, got (%d,%d,%d)", completed, total, percentage)
	}
}

func TestRunAllChecks(t *testing.T) {
	updateCallCount := 0
	updateFunc := func() { updateCallCount++ }

	cm := NewCheckManager(updateFunc, 2)

	executed := make([]bool, 3)
	var mu sync.Mutex

	for i := range 3 {
		index := i
		localTestFunc := func(reporter SubProgressReporter) error {
			reporter.ReportSubProgress(0, "Starting...")
			mu.Lock()
			executed[index] = true
			mu.Unlock()
			time.Sleep(50 * time.Millisecond) // Simulate work
			reporter.ReportSubProgress(100, "Completed")
			return nil
		}
		cm.AddCheck("check", localTestFunc)
	}

	cm.RunAllChecks()

	// Wait for all checks to complete
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	for i, exec := range executed {
		if !exec {
			t.Errorf("Check %d was not executed", i)
		}
	}
	mu.Unlock()

	if updateCallCount == 0 {
		t.Error("UI update function was never called")
	}
}

func TestRunAllChecksOnlyPending(t *testing.T) {
	cm := NewCheckManager(nil, 1)

	executed := false
	localTestFunc := func(reporter SubProgressReporter) error {
		reporter.ReportSubProgress(0, "Starting...")
		executed = true
		reporter.ReportSubProgress(100, "Completed")
		return nil
	}

	cm.AddCheck("test", localTestFunc)

	// Mark as in progress
	cm.items[0].mu.Lock()
	cm.items[0].Status = StatusInProgress
	cm.items[0].mu.Unlock()

	cm.RunAllChecks()
	time.Sleep(100 * time.Millisecond)

	if executed {
		t.Error("Non-pending check should not have been executed")
	}
}

func TestConcurrentAccess(t *testing.T) {
	cm := NewCheckManager(nil, 10)

	var wg sync.WaitGroup

	// Concurrent adding
	for i := range 10 {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			cm.AddCheck("check", testFunc)
		}(i)
	}

	// Concurrent reading
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items := cm.GetItems()
			_ = len(items) // Use the result
		}()
	}

	wg.Wait()

	if len(cm.items) != 10 {
		t.Errorf("Expected 10 items after concurrent access, got %d", len(cm.items))
	}
}

func testFunc(reporter SubProgressReporter) error {
	reporter.ReportSubProgress(0, "Starting...")
	time.Sleep(50 * time.Millisecond)
	reporter.ReportSubProgress(100, "Completed")
	return nil
}
