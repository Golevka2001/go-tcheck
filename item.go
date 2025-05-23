package tcheck

import (
	"sync"
)

// CheckStatus represents the status of a check item.
type CheckStatus int

const (
	StatusPending CheckStatus = iota
	StatusInProgress
	StatusCompleted
	StatusFailed
)

// SubProgressReporter is an interface for check functions to report sub-progress.
type SubProgressReporter interface {
	ReportSubProgress(percentage int, message string)
}

// CheckFunc is the signature for a custom check function.
// It receives a SubProgressReporter to update its own progress.
type CheckFunc func(reporter SubProgressReporter) error

// CheckItem represents a single check to be performed.
type CheckItem struct {
	ID             int
	Name           string
	Status         CheckStatus
	SubProgress    int    // Percentage for in-progress items (0-100)
	SubMessage     string // Optional message for sub-progress
	Error          error  // Stores the error if the check failed
	runFunc        CheckFunc
	mu             sync.Mutex // For thread-safe updates to Status, SubProgress, Error
	reporterActive bool       // To ensure reporter is only used during execution
}

// NewCheckItem creates a new check item.
func NewCheckItem(id int, name string, fn CheckFunc) *CheckItem {
	return &CheckItem{
		ID:      id,
		Name:    name,
		Status:  StatusPending,
		runFunc: fn,
	}
}

// implement SubProgressReporter for CheckItem
type checkItemReporter struct {
	item *CheckItem
}

func (r *checkItemReporter) ReportSubProgress(percentage int, message string) {
	r.item.mu.Lock()
	defer r.item.mu.Unlock()
	if r.item.Status == StatusInProgress && r.item.reporterActive {
		if percentage < 0 {
			percentage = 0
		}
		if percentage > 100 {
			percentage = 100
		}
		r.item.SubProgress = percentage
		r.item.SubMessage = message
		// Here you would typically send an event to the UI to redraw this item
		// For this example, we'll just print, but in tcell you'd post an event.
		// fmt.Printf("UI Event: Update item %d - SubProgress: %d%%, Message: %s\n", r.item.ID, percentage, message)
	}
}

// Run executes the check function.
func (ci *CheckItem) Run() {
	ci.mu.Lock()
	ci.Status = StatusInProgress
	ci.SubProgress = 0
	ci.SubMessage = ""
	ci.Error = nil
	ci.reporterActive = true
	ci.mu.Unlock()

	reporter := &checkItemReporter{item: ci}
	err := ci.runFunc(reporter)

	ci.mu.Lock()
	ci.reporterActive = false
	if err != nil {
		ci.Status = StatusFailed
		ci.Error = err
	} else {
		ci.Status = StatusCompleted
		ci.SubProgress = 100 // Ensure it shows 100% on completion
	}
	ci.mu.Unlock()
}
