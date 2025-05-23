package tcheck

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
)

// UIRenderer handles the Tcell display.
type UIRenderer struct {
	screen              tcell.Screen
	manager             *CheckManager
	StyleDefault        tcell.Style
	StyleGood           tcell.Style
	StyleBad            tcell.Style
	StyleWarning        tcell.Style
	StyleScrollBar      tcell.Style
	StyleScrollBarThumb tcell.Style
	StyleScrollBarArrow tcell.Style
	StyleProgress       tcell.Style
	mu                  sync.Mutex // For screen operations
	scrollTop           int        // Top visible item index for scrolling
	quit                chan struct{}
}

// NewUIRenderer creates a new UI renderer.
func NewUIRenderer(s tcell.Screen, cm *CheckManager) *UIRenderer {
	return &UIRenderer{
		screen:              s,
		manager:             cm,
		StyleDefault:        tcell.StyleDefault.Foreground(tcell.ColorSilver).Background(tcell.ColorNone),
		StyleGood:           tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.ColorNone),
		StyleBad:            tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorNone),
		StyleWarning:        tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.ColorNone),
		StyleScrollBar:      tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Background(tcell.ColorNone),
		StyleScrollBarThumb: tcell.StyleDefault.Foreground(tcell.ColorSilver).Background(tcell.ColorNone),
		StyleScrollBarArrow: tcell.StyleDefault.Foreground(tcell.ColorSilver).Background(tcell.ColorNone),
		StyleProgress:       tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorTeal),
		scrollTop:           0,
		quit:                make(chan struct{}),
	}
}

func (ui *UIRenderer) emitStr(x, y int, style tcell.Style, str string) {
	for _, c := range str {
		ui.screen.SetContent(x, y, c, nil, style)
		x++
	}
}

// drawScrollBar draws a visual scroll bar on the right side of the screen
func (ui *UIRenderer) drawScrollBar(width, height, numItems, displayableRows int) {
	if numItems <= displayableRows {
		return
	}

	// Calculate scroll bar dimensions
	scrollBarHeight := displayableRows - 2 // Leave space for arrows
	scrollBarWidth := 1
	scrollBarX := width - scrollBarWidth

	// Calculate thumb position and size
	thumbSize := max(1, (scrollBarHeight*displayableRows)/numItems)

	// Calculate the maximum scroll position
	maxScroll := numItems - displayableRows
	// Calculate the current scroll position as a percentage
	scrollPercentage := float64(ui.scrollTop) / float64(maxScroll)
	// Calculate the thumb position based on the scroll percentage
	thumbPosition := int(float64(scrollBarHeight-thumbSize) * scrollPercentage)

	// Draw scroll bar track
	for y := 1; y < displayableRows-1; y++ {
		ui.emitStr(scrollBarX, y, ui.StyleScrollBar, "│")
	}

	// Draw scroll bar thumb
	for y := 0; y < thumbSize; y++ {
		pos := thumbPosition + y + 1 // +1 to account for top arrow
		if pos < displayableRows-1 {
			ui.emitStr(scrollBarX, pos, ui.StyleScrollBarThumb, "█")
		}
	}
}

// Draw renders the entire UI.
func (ui *UIRenderer) Draw() {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	ui.screen.Clear()
	width, height := ui.screen.Size()

	if height < 3 {
		ui.emitStr(0, 0, ui.StyleBad, "Screen too small!")
		ui.screen.Show()
		return
	}

	items := ui.manager.GetItems()
	numItems := len(items)
	displayableRows := height - 1

	// Handle scrolling
	if ui.scrollTop > 0 && ui.scrollTop >= numItems-displayableRows+1 && numItems > displayableRows {
		ui.scrollTop = max(numItems-displayableRows, 0)
	}

	// Draw items
	y := 0
	for i := ui.scrollTop; i < numItems && y < displayableRows; i++ {
		item := items[i]
		item.mu.Lock()
		status := item.Status
		name := item.Name
		subProgress := item.SubProgress
		subMessage := item.SubMessage
		err := item.Error
		item.mu.Unlock()

		var line string
		style := ui.StyleDefault

		switch status {
		case StatusCompleted:
			style = ui.StyleGood
			line = fmt.Sprintf("✅ %s", name)
		case StatusFailed:
			style = ui.StyleBad
			errMsg := ""
			if err != nil {
				errMsg = fmt.Sprintf(" (%s)", err.Error())
			}
			line = fmt.Sprintf("❌ %s%s", name, errMsg)
		case StatusInProgress:
			style = ui.StyleWarning
			progressText := fmt.Sprintf("%d%%", subProgress)
			if subMessage != "" {
				progressText = fmt.Sprintf("%d%% - %s", subProgress, subMessage)
			}
			line = fmt.Sprintf("⏳ %s (%s)", name, progressText)
		case StatusPending:
			line = fmt.Sprintf("- %s", name)
		}
		ui.emitStr(0, y, style, line)
		y++
	}

	// Draw scroll indicators if necessary
	if displayableRows < numItems {
		if ui.scrollTop > 0 {
			ui.emitStr(width-1, 0, ui.StyleScrollBarArrow, "▲")
		}
		if ui.scrollTop+displayableRows < numItems {
			ui.emitStr(width-1, displayableRows-1, ui.StyleScrollBarArrow, "▼")
		}
	}

	// Draw scroll bar
	ui.drawScrollBar(width, height, numItems, displayableRows)

	// Draw overall progress bar at the bottom
	completed, total, overallProgress := ui.manager.CalculateOverallProgress()
	progressText := fmt.Sprintf("Overall Progress: %d/%d (%d%%)", completed, total, overallProgress)
	barWidth := width - 2 // for borders [ and ]
	filledWidth := (barWidth * overallProgress) / 100

	var sb strings.Builder
	sb.WriteString("[")
	for i := range barWidth {
		if i < filledWidth {
			sb.WriteString("=") // Or a block character
		} else {
			sb.WriteString(" ")
		}
	}
	sb.WriteString("]")

	// Clear the progress bar line before drawing
	for i := range width {
		ui.screen.SetContent(i, height-1, ' ', nil, ui.StyleDefault)
	}
	ui.emitStr(0, height-1, ui.StyleDefault, sb.String())
	ui.emitStr((width-len(progressText))/2, height-1, ui.StyleProgress, progressText)

	ui.screen.Show()
}

// Run a loop to handle key presses and window resizing.
func (ui *UIRenderer) Run() {
	defer func() {
		if r := recover(); r != nil {
			ui.screen.Fini()
			log.Fatalf("UI panicked: %v", r)
		}
	}()

	// Initial draw
	ui.Draw()

	// Event loop
	go func() {
		for {
			select {
			case <-ui.quit:
				return
			default:
				ev := ui.screen.PollEvent()
				if ev == nil {
					continue
				}

				switch ev := ev.(type) {
				case *tcell.EventResize:
					ui.mu.Lock()
					ui.screen.Sync()
					ui.mu.Unlock()
					ui.Draw()
				case *tcell.EventKey:
					if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC || (ev.Key() == tcell.KeyRune && ev.Rune() == 'q') {
						close(ui.quit)
						return
					}
					if ev.Key() == tcell.KeyDown {
						ui.mu.Lock()
						itemsCount := len(ui.manager.GetItems())
						_, h := ui.screen.Size()
						displayableRows := h - 1
						if ui.scrollTop < itemsCount-displayableRows {
							ui.scrollTop++
						}
						ui.mu.Unlock()
						ui.Draw()
					}
					if ev.Key() == tcell.KeyUp {
						ui.mu.Lock()
						if ui.scrollTop > 0 {
							ui.scrollTop--
						}
						ui.mu.Unlock()
						ui.Draw()
					}
				}
			}
		}
	}()

	// Redraw loop (triggered by CheckManager or periodically)
	// The CheckManager's uiUpdate callback will call ui.Draw()
	// We also need this loop to handle the quit signal correctly.
	<-ui.quit
	ui.screen.Fini()
	fmt.Println("Application quit.")
}

// Stop cleanly shuts down the UI event loop.
func (ui *UIRenderer) Stop() {
	close(ui.quit)
}
