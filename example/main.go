package main

import (
	"fmt"
	"log"
	"os"
	"time"

	tcheck "github.com/Golevka2001/go-tcheck"

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

	// If the screen won't be used anymore, we can clean it up.
	ui.Stop()
	time.Sleep(100 * time.Millisecond) // Sleep for a bit to allow the UI to finish drawing.
	s.Fini()

	// Collect failed checks
	failed := []string{}
	for _, item := range manager.GetItems() {
		if item.Status == tcheck.StatusFailed {
			// Collect the information of failed checks
			failed = append(failed, fmt.Sprintf("%s: %v", item.Name, item.Error))
		}
	}

	if len(failed) > 0 {
		// If any check failed, do something...
		fmt.Println("❌ Exiting due to failed checks:")
		for _, fail := range failed {
			fmt.Printf(" - %s\n", fail)
		}
		os.Exit(1)
	}

	// If all checks passed, continue with the next steps
	fmt.Println("✅ All checks passed! Moving to the next step...")
	fmt.Println("Welcome!")
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
