package tcheck

import (
	"sync"
	"time"
)

// CheckManager manages a list of check items and their execution.
type CheckManager struct {
	items         []*CheckItem
	mu            sync.RWMutex
	itemCounter   int
	uiUpdate      func() // Callback to trigger UI redraw
	activeWorkers chan struct{}
}

// NewCheckManager creates a new CheckManager.
// maxConcurrentChecks limits how many checks run at the same time.
func NewCheckManager(uiUpdateFunc func(), maxConcurrentChecks int) *CheckManager {
	maxConcurrentChecks = max(maxConcurrentChecks, 1) // Default to at least one worker

	return &CheckManager{
		items:         make([]*CheckItem, 0),
		uiUpdate:      uiUpdateFunc,
		activeWorkers: make(chan struct{}, maxConcurrentChecks),
	}
}

// AddCheck adds a new check to the manager.
func (cm *CheckManager) AddCheck(name string, fn CheckFunc) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.itemCounter++
	item := NewCheckItem(cm.itemCounter, name, fn)
	cm.items = append(cm.items, item)
}

// GetItems returns a thread-safe copy of the check items.
func (cm *CheckManager) GetItems() []*CheckItem {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Avoid modification issues if the caller holds onto it
	itemsCopy := make([]*CheckItem, len(cm.items))
	copy(itemsCopy, cm.items)
	return itemsCopy
}

// RunAllChecks starts executing all pending checks.
func (cm *CheckManager) RunAllChecks() {
	itemsToRun := cm.GetItems() // Get a snapshot of items to run

	var wg sync.WaitGroup
	for _, item := range itemsToRun {
		// Check if the item is pending before running
		item.mu.Lock()
		isPending := item.Status == StatusPending
		item.mu.Unlock()

		if isPending {
			wg.Add(1)
			cm.activeWorkers <- struct{}{} // Acquire a worker slot

			go func(check *CheckItem) {
				defer wg.Done()
				defer func() { <-cm.activeWorkers }() // Release worker slot

				check.Run()
				if cm.uiUpdate != nil {
					cm.uiUpdate() // Signal UI to redraw after a check completes
				}
			}(item)
		}
	}

	// Periodically update UI for sub-progress, even if not all checks are done
	// This is a simple approach; a more sophisticated one might use channels
	// from each CheckItem to signal sub-progress updates.
	go func() {
		for {
			allDone := true
			items := cm.GetItems()
			for _, item := range items {
				item.mu.Lock()
				status := item.Status
				item.mu.Unlock()
				if status == StatusPending || status == StatusInProgress {
					allDone = false
					break
				}
			}

			if cm.uiUpdate != nil {
				cm.uiUpdate()
			}

			if allDone {
				// One final update
				if cm.uiUpdate != nil {
					cm.uiUpdate()
				}
				return
			}
			time.Sleep(100 * time.Millisecond) // UI refresh rate for sub-progress
		}
	}()

	// wg.Wait() // Optionally wait for all to complete if RunAllChecks should be blocking
}

// CalculateOverallProgress calculates the overall progress percentage.
func (cm *CheckManager) CalculateOverallProgress() (int, int, int) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if len(cm.items) == 0 {
		return 0, 0, 0
	}

	completedCount := 0
	for _, item := range cm.items {
		item.mu.Lock()
		if item.Status == StatusCompleted || item.Status == StatusFailed {
			completedCount++
		}
		item.mu.Unlock()
	}
	totalCount := len(cm.items)
	return completedCount, totalCount, (completedCount * 100) / totalCount
}
