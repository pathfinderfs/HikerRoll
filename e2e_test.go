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
	// initDB("./test.db")   // Switching to this can be helpful for debugging
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
	time.Sleep(1 * time.Second) // Server startup wait

	path, found := launcher.LookPath()
	if !found {
		log.Println("Browser binary not found, attempting to download...")
		ex, err := launcher.NewBrowser().Get()
		if err != nil {
			log.Fatalf("Failed to download browser binary: %v", err)
		}
		path = ex
		log.Printf("Browser binary downloaded to: %s", path)
	}

	l := launcher.New().Bin(path)
	// Ensure E2E tests run in headless mode for CI/sandbox environments
	l.Headless(false).NoSandbox(true)

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

	if _, errActivate := page.Activate(); errActivate != nil {
		t.Logf("isElementVisible: Warning - could not activate page for selector '%s': %v", selector, errActivate)
	}

	_, errDo := page.Timeout(timeout).Race().Element(selector).MustHandle(func(e *rod.Element) {
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

// uses runes to handle multi-byte characters correctly
func truncateString(s string, num int) string {
	runes := []rune(s)
	if len(runes) <= num {
		return s
	}
	return string(runes[0:num]) + "..."
}

// Navigate to create hike page and fill out the form
func createHike(t *testing.T, page *rod.Page, hikeName, organization, trailheadName, leaderName, leaderPhone string) {
	// Click the Create Hike button
	assert.True(t, isElementVisible(t, page, "button[onclick='showCreateHikePage()']", 10*time.Second), "Create New Hike button")
	page.MustElement("button[onclick='showCreateHikePage()']").MustClick()

	// Fill out hike information
	assert.True(t, isElementVisible(t, page, "#create-hike-page", 5*time.Second), "Create hike page")
	page.MustElement("#hike-name").MustInput(hikeName)
	page.MustElement("#hike-organization").MustInput(organization)
	page.MustElement("#hike-trailheadName").MustInput(trailheadName[:2])

	// This is to handle the autocomplete selection for trailname
	assert.True(t, isElementVisible(t, page, ".autocomplete-items", 3*time.Second), "Autocomplete items container")

	page.MustElementByJS(fmt.Sprintf(`
	() => {
		const items = document.querySelectorAll('.autocomplete-items div');
		for (let item of items) {
			if (item.textContent.includes(%q)) return item;
		}
		return null;
	}`, trailheadName)).MustClick()

	page.MustElement("#leader-name").MustInput(leaderName)
	page.MustElement("#leader-phone").MustInput(leaderPhone)

	// Fill out a hike time of 24 hours from now
	tomorrow := time.Now().Add(24 * time.Hour)
	targetYear := tomorrow.Year()
	targetMonth := tomorrow.Month().String() // This is time.Month (1-12)

	// Open the date time picker
	page.MustElement("input[placeholder='Click to select date and time'][type='text']").MustClick()
	assert.True(t, isElementVisible(t, page, ".flatpickr-calendar.open", 5*time.Second), "Flatpickr calendar")

	monthEl := page.MustElement(".flatpickr-monthDropdown-months")
	monthEl.MustSelect(targetMonth)
	t.Logf("Set month definitively to %s", targetMonth)

	yearEl := page.MustElement(".flatpickr-current-month .numInput.cur-year")
	yearEl.MustSelectAllText().MustInput(fmt.Sprintf("%d", targetYear)).MustType(input.Enter)
	t.Logf("Set year definitively to %d", targetYear)

	daySelector := fmt.Sprintf(".flatpickr-day:not(.prevMonthDay):not(.nextMonthDay)[aria-label*='%s'][aria-label*='%d']", tomorrow.Format("January"), tomorrow.Day())
	assert.True(t, isElementVisible(t, page, daySelector, 5*time.Second), "Flatpickr day")
	page.MustElement(daySelector).MustClick()

	// AM/PM Time setting logic
	targetHour24 := tomorrow.Hour()
	targetMinute := tomorrow.Minute()
	targetIsPM := targetHour24 >= 12
	hour12 := targetHour24 % 12
	if hour12 == 0 { // Midnight (00:xx) or Noon (12:xx)
		hour12 = 12
	}

	hourInputEl := page.MustElement(".flatpickr-time .numInput.flatpickr-hour")
	hourInputEl.MustSelectAllText().MustInput(fmt.Sprintf("%d", hour12)) // Input 12-hour format, no leading zero

	minuteInputEl := page.MustElement(".flatpickr-time .numInput.flatpickr-minute")
	minuteInputEl.MustSelectAllText().MustInput(fmt.Sprintf("%02d", targetMinute))

	// Check current AM/PM state and toggle if necessary
	// Common Flatpickr selector for the AM/PM toggle/display element
	amPmElement := page.MustElement(".flatpickr-time .flatpickr-am-pm")
	currentAmPmState := strings.ToUpper(amPmElement.MustText())
	desiredAmPmState := "AM"
	if targetIsPM {
		desiredAmPmState = "PM"
	}

	if currentAmPmState != desiredAmPmState {
		amPmElement.MustClick() // Click to toggle
		t.Logf("Toggled AM/PM to %s", desiredAmPmState)
	}

	// It's good practice to ensure the picker closes or focus moves away.
	// Clicking the minute input or typing Enter there often helps.
	minuteInputEl.MustType(input.Enter)

	page.MustElement("#create-hike-form button[onclick='createHike()']").MustClick()
	assert.True(t, isElementVisible(t, page, "#hike-leader-page", 10*time.Second), "Hike leader page")
	t.Log("Hike Leader: Successfully on Coordinator Console page")
}

func joinHike(t *testing.T, page *rod.Page, joinCode, participantName, participantPhone, licensePlate, emergencyContact string) *rod.Page {
	t.Log("Participant: Starting to join hike...")
	participantPage := testBrowser.MustIncognito().MustPage()
	participantJoinURL := fmt.Sprintf("%s/?code=%s", baseServerURL, joinCode)
	participantPage.MustNavigate(participantJoinURL).MustWaitLoad()

	assert.True(t, isElementVisible(t, participantPage, "#join-hike-page", 10*time.Second), "Join hike page for participant")
	participantPage.MustElement("#participant-name").MustInput(participantName)
	participantPage.MustElement("#participant-phone").MustInput(participantPhone)
	participantPage.MustElement("#participant-licensePlate").MustInput(licensePlate)
	participantPage.MustElement("#participant-emergencyContact").MustInput(emergencyContact)
	participantPage.MustElement("#join-hike-form button[onclick='showWaiverPage()']").MustClick()

	assert.True(t, isElementVisible(t, participantPage, "#waiver-page", 5*time.Second), "Waiver page")
	waiverContentSelector := "#waiver-content"
	assert.True(t, isElementVisible(t, participantPage, waiverContentSelector, 5*time.Second), "Waiver content")
	// waiverText := participantPage.MustElement(waiverContentSelector).MustText()
	// assert.Contains(t, strings.ToLower(waiverText), "waiver", "Waiver text problem")
	participantPage.MustElement("#waiver-page button[onclick='joinHike()']").MustClick()

	// Wait for the welcome page to show and the specific RSVPed hike to be listed
	assert.True(t, isElementVisible(t, participantPage, "#welcome-page", 10*time.Second), "Welcome page after RSVP")

	// XPath to find the specific li for the RSVPed hike
	rsvpedHikeItemXPath := fmt.Sprintf("//ul[@id='rsvped-hikes-list']/li[.//h3[contains(normalize-space(.), 'E2E Test Hike')] and .//button[contains(@onclick, \"startHiking('%s'\")]]", joinCode)

	// Wait for the list item itself to be visible using the XPath
	participantPage.Timeout(15 * time.Second).MustElementX(rsvpedHikeItemXPath).MustWaitVisible()
	t.Log("RSVPed hike 'E2E Test Hike' in list is visible") // Log success

	// Click the "Start Hiking" button within this specific list item
	// The button can be found relative to the rsvpedHikeItemXPath or as a direct child.
	// XPath for the button within that li:
	startHikingButtonXPath := fmt.Sprintf("%s//button[contains(@onclick, 'startHiking')]", rsvpedHikeItemXPath)
	participantPage.MustElementX(startHikingButtonXPath).MustClick()

	assert.True(t, isElementVisible(t, participantPage, "#hiking-page", 10*time.Second), "Hiking page for participant")
	t.Log("Participant: Successfully on Hiking page")
	return participantPage
}

func TestHikeLifecycle(t *testing.T) {
	leaderPage := testBrowser.MustPage(baseServerURL).MustWaitLoad()

	defer leaderPage.MustClose()
	t.Log("Hike Leader: Starting to create hike...")

	createHike(t, leaderPage, "E2E Test Hike", "E2E Test Organization", "Ka'au Crater", "E2E Leader", "1234567890")

	// Get join URL from the leader page
	joinURLElement := leaderPage.MustElement("#join-url")
	joinURLString, _ := joinURLElement.Attribute("href")
	parsedJoinURL, _ := url.Parse(*joinURLString)
	joinCode := parsedJoinURL.Query().Get("code")
	t.Logf("Hike Leader: Extracted joinCode: %s", joinCode)

	// Leader: Navigate back to Welcome Page to check "Hikes I'm Leading"
	t.Log("Hike Leader: Navigating to Welcome Page to check 'Leading' section...")
	leaderPage.MustElementX(`//button[@onclick="goHomeFromLeaderConsole()"]`).MustClick()

	//leaderPage.MustNavigate(baseServerURL).MustWaitLoad() // Simulate going back to welcome page
	assert.True(t, isElementVisible(t, leaderPage, "#welcome-page", 10*time.Second), "Welcome page for leader")

	// Assert "Hikes I'm Leading" section title using XPath
	hikesImLeadingTitleXPath := "//h2[contains(normalize-space(.), \"Leading\")]"
	assert.True(t, leaderPage.MustHasX(hikesImLeadingTitleXPath), "Section 'Leading' title found")

	// XPath for the specific "E2E Test Hike" in the "Leading list
	leadingHikeItemXPath := "//ul[@id='leading-hikes-list']/li[.//h3[contains(normalize-space(.), 'E2E Test Hike')]]"
	leaderPage.Timeout(15 * time.Second).MustElementX(leadingHikeItemXPath).MustWaitVisible()
	t.Log("Created hike 'E2E Test Hike' in 'Hikes I'm Leading' list is visible") // Log success

	t.Log("Hike Leader: Clicking 'Open Coordinator Console' from 'Leading' list to return to console...")
	goToConsoleButtonXPath := fmt.Sprintf("%s//button[contains(@onclick, 'goToLeaderConsole')]", leadingHikeItemXPath)
	leaderPage.MustElementX(goToConsoleButtonXPath).MustClick()

	assert.True(t, isElementVisible(t, leaderPage, "#hike-leader-page", 10*time.Second), "Hike leader page (re-accessed)")
	t.Log("Hike Leader: Successfully back on Coordinator Console page")

	participantPage := joinHike(t, leaderPage, joinCode, "E2E Participant", "0987654321", "E2E-PLATE", "5555555555")
	defer participantPage.MustClose()

	t.Log("Hike Leader: Checking for participant...")
	if _, errActivate := leaderPage.Activate(); errActivate != nil {
		t.Logf("Warning: could not activate leaderPage: %v", errActivate)
	}
	leaderPage.MustElement("button[onclick='refreshParticipants()']").MustClick()

	assert.Eventually(t, func() bool {
		if !isElementVisible(t, leaderPage, "#participant-list", 2*time.Second) {
			return false
		}
		elements, errEls := leaderPage.Elements("#participant-list tr")
		if errEls != nil || len(elements) == 0 {
			leaderPage.MustElement("button[onclick='refreshParticipants()']").MustClick()
			return false
		}
		for _, el := range elements {
			nameCell, errN := el.Element("td:first-child a")
			phoneCell, errP := el.Element("td:nth-child(2) a")
			if errN != nil || errP != nil {
				continue
			}
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
		if len(elements) == 0 {
			return true
		}
		for _, el := range elements {
			nameCell, errN := el.Element("td:first-child a")
			if errN != nil {
				continue
			}
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
