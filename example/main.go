package main

import (
	"fmt"
	"log"
	"time"

	tcheck "go-tcheck"

	"github.com/gdamore/tcell/v2"
)

func main() {
	// Initialize tcell screen
	s, err := tcell.NewScreen()
	if err != nil {
		log.Fatalf("Failed to create screen: %v", err)
	}
	if err := s.Init(); err != nil {
		log.Fatalf("Failed to initialize screen: %v", err)
	}
	defer s.Fini()

	// Create CheckManager and UIRenderer
	// The UIRenderer needs a way to be told to redraw when item states change.
	// We'll use a simple function reference for this.
	var ui *tcheck.UIRenderer
	manager := tcheck.NewCheckManager(func() {
		if ui != nil {
			ui.Draw() // Trigger a redraw
		}
	}, 3) // Allow up to 3 checks to run concurrently

	ui = tcheck.NewUIRenderer(s, manager)

	// --- How to Add Custom Check Functions ---
	manager.AddCheck("Checking Network Connectivity", func(reporter tcheck.SubProgressReporter) error {
		reporter.ReportSubProgress(0, "Pinging gateway...")
		time.Sleep(1 * time.Second)
		// Simulate ping success/failure
		if time.Now().Second()%2 == 0 {
			reporter.ReportSubProgress(100, "Gateway reachable")
			return nil
		}
		reporter.ReportSubProgress(100, "Gateway ping failed")
		return fmt.Errorf("gateway not reachable")
	})

	manager.AddCheck("Verifying File Permissions", ExampleCheckSuccessful) // Using a predefined function
	manager.AddCheck("Checking Database Connection", ExampleCheckFailed)
	manager.AddCheck("System Resource Check", func(reporter tcheck.SubProgressReporter) error {
		reporter.ReportSubProgress(10, "Checking CPU...")
		time.Sleep(300 * time.Millisecond)
		reporter.ReportSubProgress(50, "Checking Memory...")
		time.Sleep(500 * time.Millisecond)
		reporter.ReportSubProgress(100, "Resources OK")
		return nil
	})
	manager.AddCheck("External API Availability", ExampleCheckLongNoSubProgress)
	manager.AddCheck("Configuration File Syntax", ExampleCheckQuick)
	manager.AddCheck("Disk Space Check", func(reporter tcheck.SubProgressReporter) error {
		totalSteps := 5
		for i := 0; i <= totalSteps; i++ {
			reporter.ReportSubProgress((i*100)/totalSteps, fmt.Sprintf("Analyzing partition %d/%d", i, totalSteps))
			time.Sleep(300 * time.Millisecond)
		}
		return nil
	})
	manager.AddCheck("Another Successful Check", ExampleCheckSuccessful)
	manager.AddCheck("Yet Another Failing Check", ExampleCheckFailed)
	manager.AddCheck("Quick Pass", ExampleCheckQuick)

	// Start running checks in the background
	go manager.RunAllChecks()

	// Start the UI event loop (this will block until quit)
	ui.Run()

	// After UI loop exits (e.g., user presses 'q')
	// You might want to wait for any remaining checks if RunAllChecks was non-blocking
	// and you want to ensure cleanup or logging of all results.
	// For this example, RunAllChecks spawns goroutines and doesn't block main.
	// The UIRenderer's quit signal will stop the UI, and the application will then exit.
	// If checks are still running, their goroutines will continue until they complete.
	fmt.Println("Checks processing initiated. UI has been closed.")
	// Allow a moment for any final check updates if they were very close to finishing
	// In a real app, you'd use sync.WaitGroup if you needed to ensure all checks complete
	// before the program fully exits.
	time.Sleep(1 * time.Second)
	log.Println("Exiting application.")
}

// ExampleCheckSuccessful demonstrates a check that completes successfully.
func ExampleCheckSuccessful(reporter tcheck.SubProgressReporter) error {
	reporter.ReportSubProgress(0, "Starting...")
	time.Sleep(500 * time.Millisecond) // Simulate work

	reporter.ReportSubProgress(30, "Doing step 1/3")
	time.Sleep(1 * time.Second)

	reporter.ReportSubProgress(60, "Doing step 2/3")
	time.Sleep(1 * time.Second)

	reporter.ReportSubProgress(90, "Almost done with step 3/3")
	time.Sleep(500 * time.Millisecond)

	// reporter.ReportSubProgress(100, "Completed") // Manager will set to 100% on success
	return nil
}

// ExampleCheckFailed demonstrates a check that fails.
func ExampleCheckFailed(reporter tcheck.SubProgressReporter) error {
	reporter.ReportSubProgress(0, "Attempting critical operation...")
	time.Sleep(1 * time.Second)
	reporter.ReportSubProgress(50, "Operation in progress...")
	time.Sleep(1 * time.Second)
	return fmt.Errorf("simulated failure: resource not available")
}

// ExampleCheckQuick demonstrates a quick check.
func ExampleCheckQuick(reporter tcheck.SubProgressReporter) error {
	// This check is too fast to report sub-progress meaningfully, but we can.
	reporter.ReportSubProgress(50, "Verifying...")
	time.Sleep(200 * time.Millisecond)
	return nil
}

// ExampleCheckLongNoSubProgress demonstrates a check that takes time but doesn't report sub-progress.
func ExampleCheckLongNoSubProgress(reporter tcheck.SubProgressReporter) error {
	// Even if you don't have distinct sub-steps, you can report initial/final messages.
	reporter.ReportSubProgress(0, "Performing lengthy operation...")
	time.Sleep(3 * time.Second) // Simulate long work
	// No further sub-progress reported.
	return nil
}
