package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

const baseServerURL = "http://localhost:8197"
var testBrowser *rod.Browser

func TestMain(m *testing.M) {
	initDB(":memory:")
	defer func() {
		if db != nil {
			db.Close()
		}
	}()

	mux := http.NewServeMux()
	addRoutes(mux)
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/", fs)
	server := &http.Server{Addr: ":8197", Handler: mux}

	go func() {
		log.Println("E2E Test server starting on :8197")
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("E2E Test server ListenAndServe error: %v", err)
		}
	}()
	time.Sleep(2 * time.Second) // Server startup wait

	path, موجود := launcher.LookPath()
	if !موجود {
		log.Println("Browser binary not found, attempting to download...")
		ex, err := launcher.NewBrowser().Get()
		if err != nil {
			log.Fatalf("Failed to download browser binary: %v", err)
		}
		path = ex
		log.Printf("Browser binary downloaded to: %s", path)
	}

	l := launcher.New().Bin(path)
	l.Headless(true).NoSandbox(true)

	controlURL, errLaunch := l.Launch()
	if errLaunch != nil {
		log.Fatalf("Failed to launch browser: %v", errLaunch)
	}

	testBrowser = rod.New().ControlURL(controlURL).MustConnect()

	defer func() {
		log.Println("Closing browser...")
		errClose := testBrowser.Close()
		if errClose != nil {
			log.Printf("Error closing browser: %v", errClose)
		}
	}()

	code := m.Run()
	os.Exit(code)
}

func isElementVisible(t *testing.T, page *rod.Page, selector string, timeout time.Duration) bool {
	t.Helper()
	// If Activate returns two values (e.g. (*Page, error)) in this Rod version:
	// if _, errActivate := page.Activate(); errActivate != nil {
	// Otherwise, if it only returns error:
	if errActivate := page.Activate(); errActivate != nil {
		t.Logf("isElementVisible: Warning - could not activate page for selector '%s': %v", selector, errActivate)
	}

	// If Do returns two values (e.g. (Value, error)) in this Rod version:
	// _, errDo := page.Timeout(timeout).Race().Element(selector).MustHandle(func(e *rod.Element) { ... }).Do()
	// If it only returns error:
	errDo := page.Timeout(timeout).Race().Element(selector).MustHandle(func(e *rod.Element) {
		e.MustWaitVisible()
	}).Do()

	if errDo != nil {
		html, _ := page.HTML()
		pageURL := "unknown"
		if info, infoErr := page.Info(); infoErr == nil {
			pageURL = info.URL
		}
		t.Logf("isElementVisible: Error waiting for selector '%s' (timeout: %v). Current page URL: %s\nPage HTML (first 500 chars): %s\nError: %v",
			selector, timeout, pageURL, truncateString(html, 500), errDo)
		currentTimestamp := time.Now().Format("20060102_150405")
		safeSelector := strings.ReplaceAll(selector, "#", "id_")
		safeSelector = strings.ReplaceAll(safeSelector, ".", "class_")
		safeSelector = strings.ReplaceAll(safeSelector, "[", "")
		safeSelector = strings.ReplaceAll(safeSelector, "]", "")
		safeSelector = strings.ReplaceAll(safeSelector, "'", "")
		safeSelector = strings.ReplaceAll(safeSelector, "\"", "")
		safeSelector = strings.ReplaceAll(safeSelector, " ", "_")
		safeSelector = strings.ReplaceAll(safeSelector, ":", "_")
		safeSelector = strings.ReplaceAll(safeSelector, "(", "_")
		safeSelector = strings.ReplaceAll(safeSelector, ")", "_")
		safeSelector = strings.ReplaceAll(safeSelector, ">", "_")
		if len(safeSelector) > 50 {
			safeSelector = safeSelector[:50]
		}
		screenshotPath := fmt.Sprintf("debug_isElementVisible_error_%s_%s.png", safeSelector, currentTimestamp)

		screenshotsDir := "screenshots"
		if _, statErr := os.Stat(screenshotsDir); os.IsNotExist(statErr) {
			if mkdirErr := os.Mkdir(screenshotsDir, 0755); mkdirErr != nil {
				t.Logf("Failed to create screenshots directory '%s': %v", screenshotsDir, mkdirErr)
                // Attempt to save in current directory if subdir creation fails
                func() {
                    defer func() {
                        if r := recover(); r != nil {
                            t.Logf("Recovered in MustScreenshot (current dir): %v", r)
                        }
                    }()
                    page.MustScreenshot(screenshotPath)
                    t.Logf("Screenshot saved to current directory: %s", screenshotPath)
                }()
                return false
			}
		}
		fullScreenshotPath := screenshotsDir + "/" + screenshotPath
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Recovered in MustScreenshot (screenshots dir): %v", r)
				}
			}()
			page.MustScreenshot(fullScreenshotPath)
			t.Logf("Screenshot saved to %s", fullScreenshotPath)
		}()
		return false
	}
	return true
}

func truncateString(s string, num int) string {
	runes := []rune(s)
	if len(runes) <= num {
		return s
	}
	return string(runes[0:num]) + "..."
}

func TestHikeLifecycle(t *testing.T) {
	leaderPage := testBrowser.MustPage(baseServerURL).MustWaitLoad()
	defer leaderPage.MustClose()

	t.Log("Hike Leader: Starting to create hike...")
	assert.True(t, isElementVisible(t, leaderPage, "button[onclick='showCreateHikePage()']", 10*time.Second), "Create New Hike button")
	leaderPage.MustElement("button[onclick='showCreateHikePage()']").MustClick()

	assert.True(t, isElementVisible(t, leaderPage, "#create-hike-page", 5*time.Second), "Create hike page")
	leaderPage.MustElement("#hike-name").MustInput("E2E Test Hike")
	leaderPage.MustElement("#trailhead-name").MustInput("Ka")
	assert.True(t, isElementVisible(t, leaderPage, ".autocomplete-items", 3*time.Second), "Autocomplete items container")

	leaderPage.MustElementByJS(`
		() => {
			const items = document.querySelectorAll('.autocomplete-items div');
			for (let item of items) { if (item.textContent.includes("Ka'ena Point Trailhead")) return item; } return null;
		}
	`).MustClick()
	leaderPage.MustElement("#leader-name").MustInput("E2E Leader")
	leaderPage.MustElement("#leader-phone").MustInput("123-456-7890")

	tomorrow := time.Now().Add(24 * time.Hour)
	leaderPage.MustElement("input[placeholder='Click to select date and time']").MustClick()
	assert.True(t, isElementVisible(t, leaderPage, ".flatpickr-calendar.open", 5*time.Second), "Flatpickr calendar")
	yearEl := leaderPage.MustElement(".flatpickr-current-month .numInput.cur-year")
	yearEl.MustSelectAllText()
	yearEl.MustInput(fmt.Sprintf("%d", tomorrow.Year()))
	yearEl.MustKey(input.Enter)

	leaderPage.MustElement(fmt.Sprintf(".flatpickr-monthDropdown-months .flatpickr-monthDropdown-month[value='%d']", int(tomorrow.Month())-1)).MustClick()
	daySelector := fmt.Sprintf(".flatpickr-day:not(.prevMonthDay):not(.nextMonthDay)[aria-label*='%s'][aria-label*='%d']", tomorrow.Format("January"), tomorrow.Day())
	assert.True(t, isElementVisible(t, leaderPage, daySelector, 5*time.Second), "Flatpickr day")
	leaderPage.MustElement(daySelector).MustClick()
	hourEl := leaderPage.MustElement(".flatpickr-time .numInput.flatpickr-hour")
	hourEl.MustSelectAllText().MustInput(fmt.Sprintf("%02d", tomorrow.Hour()))
	minuteEl := leaderPage.MustElement(".flatpickr-time .numInput.flatpickr-minute")
	minuteEl.MustSelectAllText().MustInput(fmt.Sprintf("%02d", tomorrow.Minute()))
	if okButton, errOk := leaderPage.Element(".flatpickr-time .flatpickr-confirm"); errOk == nil {
		if isVis, _ := okButton.Visible(); isVis { okButton.MustClick() }
	}

	leaderPage.MustElement("#create-hike-form button[onclick='createHike()']").MustClick()
	assert.True(t, isElementVisible(t, leaderPage, "#hike-leader-page", 10*time.Second), "Hike leader page")
	t.Log("Hike Leader: Successfully on Coordinator Console page")

	joinURLElement := leaderPage.MustElement("#join-url")
	joinURLString, _ := joinURLElement.Attribute("href")
	parsedJoinURL, _ := url.Parse(*joinURLString)
	joinCode := parsedJoinURL.Query().Get("code")
	t.Logf("Hike Leader: Extracted joinCode: %s", joinCode)

	t.Log("Participant: Starting to join hike...")
	participantPage := testBrowser.MustIncognito().MustPage()
	defer participantPage.MustClose()
	participantJoinURL := fmt.Sprintf("%s/?code=%s", baseServerURL, joinCode)
	participantPage.MustNavigate(participantJoinURL).MustWaitLoad()

	assert.True(t, isElementVisible(t, participantPage, "#join-hike-page", 10*time.Second), "Join hike page for participant")
	participantPage.MustElement("#participant-name").MustInput("E2E Participant")
	participantPage.MustElement("#participant-phone").MustInput("098-765-4321")
	participantPage.MustElement("#participant-licensePlate").MustInput("E2E-PLATE")
	participantPage.MustElement("#participant-emergencyContact").MustInput("555-555-5555")
	participantPage.MustElement("#join-hike-form button[onclick='showWaiverPage()']").MustClick()

	assert.True(t, isElementVisible(t, participantPage, "#waiver-page", 5*time.Second), "Waiver page")
	waiverContentSelector := "#waiver-content"
	assert.True(t, isElementVisible(t, participantPage, waiverContentSelector, 5*time.Second), "Waiver content")
	waiverText := participantPage.MustElement(waiverContentSelector).MustText()
	assert.Contains(t, strings.ToLower(waiverText), "waiver", "Waiver text problem")
	participantPage.MustElement("#waiver-page button[onclick='joinHike()']").MustClick()

	assert.True(t, isElementVisible(t, participantPage, "#hiking-page", 10*time.Second), "Hiking page for participant")
	t.Log("Participant: Successfully on Hiking page")

	t.Log("Hike Leader: Checking for participant...")
	// Corrected Activate call for leaderPage if it returns two values
	if _, errActivate := leaderPage.Activate(); errActivate != nil {
		t.Logf("Warning: could not activate leaderPage: %v", errActivate)
	}
	leaderPage.MustElement("button[onclick='refreshParticipants()']").MustClick()

	assert.Eventually(t, func() bool {
		if !isElementVisible(t, leaderPage, "#participant-list", 2*time.Second) { return false }
		elements, errEls := leaderPage.Elements("#participant-list tr")
		if errEls != nil || len(elements) == 0 {
			leaderPage.MustElement("button[onclick='refreshParticipants()']").MustClick()
			return false
		}
		for _, el := range elements {
			nameCell, errN := el.Element("td:first-child a")
			phoneCell, errP := el.Element("td:nth-child(2) a")
			if errN != nil || errP != nil { continue }
			name, _ := nameCell.Text()
			phone, _ := phoneCell.Text()
			if strings.Contains(name, "E2E Participant") && strings.Contains(phone, "098-765-4321") {
				return true
			}
		}
		leaderPage.MustElement("button[onclick='refreshParticipants()']").MustClick()
		return false
	}, 15*time.Second, 2*time.Second, "Participant 'E2E Participant' should appear")
	t.Log("Hike Leader: Participant 'E2E Participant' is visible.")

	t.Log("Participant: Leaving hike...")
	if _, errActivate := participantPage.Activate(); errActivate != nil {
		t.Logf("Warning: could not activate participantPage: %v", errActivate)
	}
	assert.True(t, isElementVisible(t, participantPage, "button[onclick='leaveHike()']", 5*time.Second), "Leave Hike button")
	participantPage.MustElement("button[onclick='leaveHike()']").MustClick()
	assert.True(t, isElementVisible(t, participantPage, "#welcome-page", 10*time.Second), "Welcome page after leaving")
	t.Log("Participant: Successfully left hike.")

	t.Log("Hike Leader: Checking if participant left...")
	if _, errActivate := leaderPage.Activate(); errActivate != nil {
		t.Logf("Warning: could not activate leaderPage: %v", errActivate)
	}
	leaderPage.MustElement("button[onclick='refreshParticipants()']").MustClick()

	assert.Eventually(t, func() bool {
		elements, errEls := leaderPage.Elements("#participant-list tr")
		if errEls != nil {
			leaderPage.MustElement("button[onclick='refreshParticipants()']").MustClick()
			return false
		}
		if len(elements) == 0 { return true }
		for _, el := range elements {
			nameCell, errN := el.Element("td:first-child a")
			if errN != nil { continue }
			name, _ := nameCell.Text()
			if strings.Contains(name, "E2E Participant") {
				leaderPage.MustElement("button[onclick='refreshParticipants()']").MustClick()
				return false
			}
		}
		return true
	}, 15*time.Second, 2*time.Second, "Participant 'E2E Participant' should be gone")
	t.Log("Hike Leader: Participant 'E2E Participant' confirmed gone from active list.")

	t.Log("Hike Leader: Ending the hike...")
	if _, errActivate := leaderPage.Activate(); errActivate != nil {
		t.Logf("Warning: could not activate leaderPage: %v", errActivate)
	}
	assert.True(t, isElementVisible(t, leaderPage, "#hike-leader-page button.button-secondary[onclick='endHike()']", 5*time.Second), "End Hike button")
	leaderPage.MustElement("#hike-leader-page button.button-secondary[onclick='endHike()']").MustClick()

	assert.True(t, isElementVisible(t, leaderPage, "#welcome-page", 10*time.Second), "Welcome page after ending hike")
	t.Log("Hike Leader: Successfully ended hike and is on welcome page.")

	t.Log("TestHikeLifecycle COMPLETED SUCCESSFULLY")
}
