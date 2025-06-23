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
	//initDB("./test.db")
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
	if _, errActivate := page.Activate(); errActivate != nil {
		t.Logf("isElementVisible: Warning - could not activate page for selector '%s': %v", selector, errActivate)
	}

	// If Do returns two values (e.g. (Value, error)) in this Rod version:
	// _, errDo := page.Timeout(timeout).Race().Element(selector).MustHandle(func(e *rod.Element) { ... }).Do()
	// If it only returns error:
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
	leaderPage.MustElement("#hike-organization").MustInput("E2E Test Organization")
	leaderPage.MustElement("#hike-trailheadName").MustInput("Ka")
	assert.True(t, isElementVisible(t, leaderPage, ".autocomplete-items", 3*time.Second), "Autocomplete items container")

	leaderPage.MustElementByJS(`
		() => {
			const items = document.querySelectorAll('.autocomplete-items div');
			for (let item of items) { if (item.textContent.includes("Ka'au Crater")) return item; } return null;
		}
	`).MustClick()
	leaderPage.MustElement("#leader-name").MustInput("E2E Leader")
	leaderPage.MustElement("#leader-phone").MustInput("1234567890")

	tomorrow := time.Now().Add(24 * time.Hour)
	leaderPage.MustElement("input[placeholder='Click to select date and time'][type='text']").MustClick()
	assert.True(t, isElementVisible(t, leaderPage, ".flatpickr-calendar.open", 5*time.Second), "Flatpickr calendar")
	yearEl := leaderPage.MustElement(".flatpickr-current-month .numInput.cur-year")
	yearEl.MustSelectAllText()
	yearEl.MustInput(fmt.Sprintf("%d", tomorrow.Year()))
	yearEl.MustType(input.Enter)

	//leaderPage.MustElement(fmt.Sprintf(".flatpickr-monthDropdown-months .flatpickr-monthDropdown-month[value='%d']", int(tomorrow.Month())-1)).MustClick()
	// monthSelector := leaderPage.MustElement("select.flatpickr-monthDropdown-months")
	// t.Log("2.5")
	// the following fails for some reason
	// monthSelector.MustSelect(fmt.Sprintf("%d", (tomorrow.Month())-1))
	// t.Log("2.7")
	// leaderPage.MustElement("select.flatpickr-monthDropdown-months").Select([fmt.Sprintf("%d", tomorrow.Month()-1)], true, "test")
	// t.Log("3")
	daySelector := fmt.Sprintf(".flatpickr-day:not(.prevMonthDay):not(.nextMonthDay)[aria-label*='%s'][aria-label*='%d']", tomorrow.Format("January"), tomorrow.Day())
	assert.True(t, isElementVisible(t, leaderPage, daySelector, 5*time.Second), "Flatpickr day")
	leaderPage.MustElement(daySelector).MustClick()
	hourEl := leaderPage.MustElement(".flatpickr-time .numInput.flatpickr-hour")
	hourEl.MustSelectAllText().MustInput(fmt.Sprintf("%02d", tomorrow.Hour()))
	minuteEl := leaderPage.MustElement(".flatpickr-time .numInput.flatpickr-minute")
	minuteEl.MustSelectAllText().MustInput(fmt.Sprintf("%02d", tomorrow.Minute()))
	minuteEl.MustType(input.Enter)

	leaderPage.MustElement("#create-hike-form button[onclick='createHike()']").MustClick()
	assert.True(t, isElementVisible(t, leaderPage, "#hike-leader-page", 10*time.Second), "Hike leader page")
	t.Log("Hike Leader: Successfully on Coordinator Console page")

	// Verify Organization is displayed on leader page
	assert.True(t, isElementVisible(t, leaderPage, "#leader-organization-display", 2*time.Second), "Organization display on leader page")
	orgTextOnLeaderPage := leaderPage.MustElement("#leader-hike-organization").MustText()
	assert.Equal(t, "E2E Test Organization", orgTextOnLeaderPage, "Organization name mismatch on leader page")
	t.Logf("Hike Leader: Verified organization '%s' is displayed", orgTextOnLeaderPage)

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
	// Verify Organization is displayed on join page
	assert.True(t, isElementVisible(t, participantPage, "#join-organization-display", 2*time.Second), "Organization display on join page")
	orgTextOnJoinPage := participantPage.MustElement("#join-hike-organization").MustText()
	assert.Equal(t, "E2E Test Organization", orgTextOnJoinPage, "Organization name mismatch on join page")
	t.Logf("Participant: Verified organization '%s' is displayed on join page", orgTextOnJoinPage)

	participantPage.MustElement("#participant-name").MustInput("E2E Participant")
	participantPage.MustElement("#participant-phone").MustInput("0987654321")
	participantPage.MustElement("#participant-licensePlate").MustInput("E2E-PLATE")
	participantPage.MustElement("#participant-emergencyContact").MustInput("5555555555")
	participantPage.MustElement("#join-hike-form button[onclick='showWaiverPage()']").MustClick()

	assert.True(t, isElementVisible(t, participantPage, "#waiver-page", 5*time.Second), "Waiver page")
	waiverContentSelector := "#waiver-content"
	assert.True(t, isElementVisible(t, participantPage, waiverContentSelector, 5*time.Second), "Waiver content")
	// waiverText := participantPage.MustElement(waiverContentSelector).MustText()
	// assert.Contains(t, strings.ToLower(waiverText), "waiver", "Waiver text problem")
	participantPage.MustElement("#waiver-page button[onclick='joinHike()']").MustClick()

	// After RSVP, user is on welcome page. Find the RSVPed hike and click "Start Hiking"
	// This assumes getRSVPedHikes will show the organization, which will be tested separately.
	// For now, just ensure the flow to get to hiking page.
	assert.True(t, isElementVisible(t, participantPage, "#welcome-page", 10*time.Second), "Welcome page after RSVP")
	// More robust selector for the "Start Hiking" button:
	hikeNameForSelector := "E2E Test Hike"
	// Find the list item that contains an h3 with the hike name, and also the specific organization
	liBaseSelector := fmt.Sprintf("//ul[@id='rsvped-hikes-list']//li[.//h3[normalize-space(text())='%s'] and .//p[contains(normalize-space(.), 'E2E Test Organization')]]", hikeNameForSelector)
	startHikingButtonSelector := liBaseSelector + "//button[normalize-space(text())='Start Hiking']"
	assert.True(t, isElementVisible(t, participantPage, startHikingButtonSelector, 10*time.Second), "Start Hiking button for RSVPed hike '"+hikeNameForSelector+"'")
	participantPage.MustElementX(startHikingButtonSelector).MustClick()


	assert.True(t, isElementVisible(t, participantPage, "#hiking-page", 10*time.Second), "Hiking page for participant")
	// Verify Organization is displayed on hiking page
	assert.True(t, isElementVisible(t, participantPage, "#hiking-organization-display", 2*time.Second), "Organization display on hiking page")
	orgTextOnHikingPage := participantPage.MustElement("#hiking-hike-organization").MustText()
	assert.Equal(t, "E2E Test Organization", orgTextOnHikingPage, "Organization name mismatch on hiking page")
	t.Logf("Participant: Verified organization '%s' is displayed on hiking page", orgTextOnHikingPage)
	t.Log("Participant: Successfully on Hiking page")

	t.Log("Hike Leader: Checking for participant...")
	// Corrected Activate call for leaderPage if it returns two values
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

func TestHikeLifecycle_NoOrganization(t *testing.T) {
	leaderPage := testBrowser.MustPage(baseServerURL).MustWaitLoad()
	defer leaderPage.MustClose()

	t.Log("NoOrg Test: Starting to create hike...")
	assert.True(t, isElementVisible(t, leaderPage, "button[onclick='showCreateHikePage()']", 10*time.Second), "Create New Hike button")
	leaderPage.MustElement("button[onclick='showCreateHikePage()']").MustClick()

	assert.True(t, isElementVisible(t, leaderPage, "#create-hike-page", 5*time.Second), "Create hike page")
	leaderPage.MustElement("#hike-name").MustInput("E2E NoOrg Hike")
	// Skip #hike-organization input
	leaderPage.MustElement("#hike-trailheadName").MustInput("Mak") // Different trailhead for uniqueness if needed
	assert.True(t, isElementVisible(t, leaderPage, ".autocomplete-items", 3*time.Second), "Autocomplete items container for Mak")

	leaderPage.MustElementByJS(`
		() => {
			const items = document.querySelectorAll('.autocomplete-items div');
			for (let item of items) { if (item.textContent.includes("Makapu'u Point Lighthouse")) return item; } return null;
		}
	`).MustClick()
	leaderPage.MustElement("#leader-name").MustInput("E2E NoOrg Leader")
	leaderPage.MustElement("#leader-phone").MustInput("1122334455")

	// Set different time to avoid clash if tests run too close
	hikeTime := time.Now().Add(25 * time.Hour)
	leaderPage.MustElement("input[placeholder='Click to select date and time'][type='text']").MustClick()
	assert.True(t, isElementVisible(t, leaderPage, ".flatpickr-calendar.open", 5*time.Second), "Flatpickr calendar")
	yearEl := leaderPage.MustElement(".flatpickr-current-month .numInput.cur-year")
	yearEl.MustSelectAllText().MustInput(fmt.Sprintf("%d", hikeTime.Year()))
	yearEl.MustType(input.Enter)
	daySelector := fmt.Sprintf(".flatpickr-day:not(.prevMonthDay):not(.nextMonthDay)[aria-label*='%s'][aria-label*='%d']", hikeTime.Format("January"), hikeTime.Day())
	assert.True(t, isElementVisible(t, leaderPage, daySelector, 5*time.Second), "Flatpickr day")
	leaderPage.MustElement(daySelector).MustClick()
	hourEl := leaderPage.MustElement(".flatpickr-time .numInput.flatpickr-hour")
	hourEl.MustSelectAllText().MustInput(fmt.Sprintf("%02d", hikeTime.Hour()))
	minuteEl := leaderPage.MustElement(".flatpickr-time .numInput.flatpickr-minute")
	minuteEl.MustSelectAllText().MustInput(fmt.Sprintf("%02d", hikeTime.Minute()))
	minuteEl.MustType(input.Enter)

	leaderPage.MustElement("#create-hike-form button[onclick='createHike()']").MustClick()
	assert.True(t, isElementVisible(t, leaderPage, "#hike-leader-page", 10*time.Second), "Hike leader page")
	t.Log("NoOrg Test: Successfully on Coordinator Console page")

	// Verify Organization is NOT displayed on leader page
	assert.False(t, leaderPage.MustHas("#leader-organization-display[style*='display: block']"), "Organization display should be hidden on leader page for NoOrg hike")
	t.Log("NoOrg Test: Verified organization is not displayed on leader page")

	joinURLElement := leaderPage.MustElement("#join-url")
	joinURLString, _ := joinURLElement.Attribute("href")
	parsedJoinURL, _ := url.Parse(*joinURLString)
	joinCode := parsedJoinURL.Query().Get("code")
	t.Logf("NoOrg Test: Extracted joinCode: %s", joinCode)

	t.Log("NoOrg Test Participant: Starting to join hike...")
	participantPage := testBrowser.MustIncognito().MustPage()
	defer participantPage.MustClose()
	participantJoinURL := fmt.Sprintf("%s/?code=%s", baseServerURL, joinCode)
	participantPage.MustNavigate(participantJoinURL).MustWaitLoad()

	assert.True(t, isElementVisible(t, participantPage, "#join-hike-page", 10*time.Second), "Join hike page for participant")
	// Verify Organization is NOT displayed on join page
	assert.False(t, participantPage.MustHas("#join-organization-display[style*='display: block']"), "Organization display should be hidden on join page for NoOrg hike")
	t.Log("NoOrg Test Participant: Verified organization is not displayed on join page")

	participantPage.MustElement("#participant-name").MustInput("E2E NoOrg Participant")
	participantPage.MustElement("#participant-phone").MustInput("2233445566")
	participantPage.MustElement("#join-hike-form button[onclick='showWaiverPage()']").MustClick()

	assert.True(t, isElementVisible(t, participantPage, "#waiver-page", 5*time.Second), "Waiver page")
	participantPage.MustElement("#waiver-page button[onclick='joinHike()']").MustClick()

	assert.True(t, isElementVisible(t, participantPage, "#welcome-page", 10*time.Second), "Welcome page after RSVP for NoOrg hike")
	hikeNameNoOrgForSelector := "E2E NoOrg Hike"
	// Find the list item that contains an h3 with the hike name. Organization is not checked here.
	liBaseSelectorNoOrg := fmt.Sprintf("//ul[@id='rsvped-hikes-list']//li[.//h3[normalize-space(text())='%s']]", hikeNameNoOrgForSelector)
	// Ensure no organization paragraph is present for this item, as an additional check of test setup
	noOrgListItemElement := participantPage.MustElementX(liBaseSelectorNoOrg)
	assert.False(t, noOrgListItemElement.MustHas("p:contains('Organization:')"), "NoOrg Hike in RSVPed list should not have an organization paragraph for this specific test.")

	startHikingButtonSelectorNoOrg := liBaseSelectorNoOrg + "//button[normalize-space(text())='Start Hiking']"
	assert.True(t, isElementVisible(t, participantPage, startHikingButtonSelectorNoOrg, 10*time.Second), "Start Hiking button for NoOrg RSVPed hike '"+hikeNameNoOrgForSelector+"'")
	participantPage.MustElementX(startHikingButtonSelectorNoOrg).MustClick()

	assert.True(t, isElementVisible(t, participantPage, "#hiking-page", 10*time.Second), "Hiking page for NoOrg participant")
	// Verify Organization is NOT displayed on hiking page
	assert.False(t, participantPage.MustHas("#hiking-organization-display[style*='display: block']"), "Organization display should be hidden on hiking page for NoOrg hike")
	t.Log("NoOrg Test Participant: Verified organization is not displayed on hiking page")
	t.Log("NoOrg Test Participant: Successfully on Hiking page")

	// Leader checks for participant
	if _, errActivate := leaderPage.Activate(); errActivate != nil {
		t.Logf("Warning: could not activate leaderPage: %v", errActivate)
	}
	leaderPage.MustElement("button[onclick='refreshParticipants()']").MustClick()
	assert.Eventually(t, func() bool {
		if !isElementVisible(t, leaderPage, "#participant-list", 2*time.Second) { return false }
		elements, _ := leaderPage.Elements("#participant-list tr td:first-child a")
		for _, el := range elements {
			if name, _ := el.Text(); strings.Contains(name, "E2E NoOrg Participant") { return true }
		}
		leaderPage.MustElement("button[onclick='refreshParticipants()']").MustClick(); return false
	}, 15*time.Second, 2*time.Second, "Participant 'E2E NoOrg Participant' should appear")
	t.Log("NoOrg Test Leader: Participant 'E2E NoOrg Participant' is visible.")


	// Participant leaves hike
	if _, errActivate := participantPage.Activate(); errActivate != nil {
		t.Logf("Warning: could not activate participantPage: %v", errActivate)
	}
	participantPage.MustElement("button[onclick='leaveHike()']").MustClick()
	assert.True(t, isElementVisible(t, participantPage, "#welcome-page", 10*time.Second), "Welcome page after leaving NoOrg hike")
	t.Log("NoOrg Test Participant: Successfully left hike.")

	// Leader ends hike
	if _, errActivate := leaderPage.Activate(); errActivate != nil {
		t.Logf("Warning: could not activate leaderPage: %v", errActivate)
	}
	leaderPage.MustElement("#hike-leader-page button.button-secondary[onclick='endHike()']").MustClick()
	assert.True(t, isElementVisible(t, leaderPage, "#welcome-page", 10*time.Second), "Welcome page after ending NoOrg hike")
	t.Log("NoOrg Test Leader: Successfully ended hike.")

	t.Log("TestHikeLifecycle_NoOrganization COMPLETED SUCCESSFULLY")
}

func TestOrganizationDisplayOnWelcomePage(t *testing.T) {
	// --- Setup: Create Hikes ---
	// Hike with Organization
	leaderOrgPage := testBrowser.MustPage(baseServerURL).MustWaitLoad()
	defer leaderOrgPage.MustClose()
	// Corrected trailheadFullName to include "(Le'ahi)"
	createHikeE2E(t, leaderOrgPage, "Org Hike Welcome", "E2E Welcome Org", "Dia", "Diamond Head Crater (Le'ahi)", "LdrWelcomeOrg", "1010101010", 26)
	// leaderOrgPage is now on the leader console for "Org Hike Welcome"
	joinURLElementOrg := leaderOrgPage.MustElement("#join-url")
	joinURLStringOrg, _ := joinURLElementOrg.Attribute("href")
	parsedJoinURLOrg, _ := url.Parse(*joinURLStringOrg)
	joinCodeOrg := parsedJoinURLOrg.Query().Get("code")
	t.Logf("Org Hike Welcome created with Join Code: %s", joinCodeOrg)
	// Close this leader's page as we'll make a new one for the next hike
	leaderOrgPage.MustClose()

	// Hike without Organization
	leaderNoOrgPage := testBrowser.MustPage(baseServerURL).MustWaitLoad()
	defer leaderNoOrgPage.MustClose()
	createHikeE2E(t, leaderNoOrgPage, "NoOrg Hike Welcome", "", "Koko", "Koko Crater (Railway)", "LdrWelcomeNoOrg", "2020202020", 27)
	joinURLElementNoOrg := leaderNoOrgPage.MustElement("#join-url")
	joinURLStringNoOrg, _ := joinURLElementNoOrg.Attribute("href")
	parsedJoinURLNoOrg, _ := url.Parse(*joinURLStringNoOrg)
	joinCodeNoOrg := parsedJoinURLNoOrg.Query().Get("code")
	t.Logf("NoOrg Hike Welcome created with Join Code: %s", joinCodeNoOrg)
	leaderNoOrgPage.MustClose()


	// --- Test: Participant views Welcome Page ---
	participantPage := testBrowser.MustPage(baseServerURL).MustWaitLoad()
	defer participantPage.MustClose()

	assert.True(t, isElementVisible(t, participantPage, "#welcome-page", 10*time.Second), "Welcome page should be visible")

	// Grant geolocation permission if prompted (specific to how Rod handles this)
	// This is a best-effort for CI. If geolocation fails, nearby hikes might not load as expected.
	// For more robust E2E, mocking geolocation or ensuring test server returns these hikes regardless of actual geo might be needed.
	go func() {
		// MustHandleDialog() returns a function to wait for the dialog, and another function to handle it.
		waitDialog, handleDialogAction := participantPage.MustHandleDialog()

		// This goroutine will block until a dialog appears.
		dialogEvent := waitDialog() // Call the first function to get the dialog event.
		t.Logf("Dialog '%s' with type '%s' appeared.", dialogEvent.Message, dialogEvent.Type)

		// Call the second function to handle the dialog (e.g., accept it).
		// The arguments are (accept bool, promptText string).
		handleDialogAction(true, "") // true to accept, "" for prompt text if it's a prompt dialog.
		t.Log("Dialog handled (accepted).")
	}()
	participantPage.MustEval(`() => {
		navigator.permissions.query = navigator.permissions.query || function(descriptor) {
			if (descriptor.name === 'geolocation') {
				return Promise.resolve({ state: 'granted' });
			}
			return Promise.reject(new TypeError("Unknown permission descriptor name."));
		};
		window.navigator.geolocation.getCurrentPosition = function(success, error) {
			success({ // Mock coordinates close to Diamond Head / Koko Head
				coords: { latitude: 21.27, longitude: -157.75, accuracy: 20 },
				timestamp: Date.now(),
			});
		};
	}`)
	participantPage.MustElement("button.button-refresh[onclick='getNearbyHikes()']").MustClick()


	// --- Verify Nearby Hikes List ---
	t.Log("Verifying organization display in Nearby Hikes list...")
	// For Org Hike
	nearbyOrgHikeSelector := fmt.Sprintf("//ul[@id='nearby-hikes']//li[contains(., 'Org Hike Welcome') and contains(., 'E2E Welcome Org') and .//button[contains(@onclick, \"%s\")]]", joinCodeOrg)
	assert.True(t, isElementVisible(t, participantPage, nearbyOrgHikeSelector, 10*time.Second), "Org Hike should be in Nearby Hikes with organization")
	t.Log("Org Hike Welcome with organization found in Nearby Hikes.")

	// For No-Org Hike (check that organization paragraph is NOT there or empty)
	// We need to find the list item first, then check its children for the organization <p>
	// This is a bit more complex with XPath if the <p> is entirely absent vs. empty.
	// Let's assume if it's there, it has text. If it's absent, MustNotHave is better.
	// For simplicity, we'll check if the text "Organization:" is present for the NoOrg hike.
	noOrgHikeListItemSelector := fmt.Sprintf("//ul[@id='nearby-hikes']//li[contains(., 'NoOrg Hike Welcome') and .//button[contains(@onclick, \"%s\")]]", joinCodeNoOrg)
	assert.True(t, isElementVisible(t, participantPage, noOrgHikeListItemSelector, 10*time.Second), "NoOrg Hike should be in Nearby Hikes")
	noOrgHikeElement := participantPage.MustElementX(noOrgHikeListItemSelector)
	assert.False(t, noOrgHikeElement.MustHas("p:contains('Organization:')"), "NoOrg Hike in Nearby list should NOT display 'Organization:' text")
	t.Log("NoOrg Hike Welcome without organization found in Nearby Hikes (and no org text displayed).")


	// --- Participant RSVPs to Org Hike ---
	t.Logf("Participant RSVPing to Org Hike Welcome (Join Code: %s)", joinCodeOrg)
	participantPage.MustElementX(nearbyOrgHikeSelector + "//button").MustClick() // Click RSVP
	assert.True(t, isElementVisible(t, participantPage, "#join-hike-page", 5*time.Second), "Join page for Org Hike")
	participantPage.MustElement("#participant-name").MustInput("E2E WelcomeUser")
	participantPage.MustElement("#participant-phone").MustInput("3030303030")
	participantPage.MustElement("#join-hike-form button[onclick='showWaiverPage()']").MustClick()
	assert.True(t, isElementVisible(t, participantPage, "#waiver-page", 5*time.Second), "Waiver page for Org Hike")
	participantPage.MustElement("#waiver-page button[onclick='joinHike()']").MustClick()
	assert.True(t, isElementVisible(t, participantPage, "#welcome-page", 10*time.Second), "Welcome page after RSVP to Org Hike")

	// --- Verify RSVPed Hikes List (Org Hike) ---
	t.Log("Verifying organization display in RSVPed Hikes list for Org Hike...")
	rsvpedOrgHikeSelector := fmt.Sprintf("//ul[@id='rsvped-hikes-list']//li[contains(., 'Org Hike Welcome') and contains(., 'E2E Welcome Org') and .//button[contains(@onclick, \"startHiking('%s')\")]]", joinCodeOrg)
	assert.True(t, isElementVisible(t, participantPage, rsvpedOrgHikeSelector, 5*time.Second), "Org Hike should be in RSVPed Hikes with organization")
	t.Log("Org Hike Welcome with organization found in RSVPed Hikes.")

	// --- Participant RSVPs to No-Org Hike ---
	// First, navigate back to welcome page and refresh nearby hikes to find the NoOrg hike again
	// (or assume it's still there if page didn't fully reload - but better to be explicit)
	participantPage.MustNavigate(baseServerURL) // Go to welcome page
	assert.True(t, isElementVisible(t, participantPage, "#welcome-page", 10*time.Second), "Welcome page re-loaded")
	participantPage.MustEval(`() => { // Re-apply mock
		navigator.permissions.query = navigator.permissions.query || function(descriptor) { /* ... */ };
		window.navigator.geolocation.getCurrentPosition = function(success, error) {
			success({ coords: { latitude: 21.27, longitude: -157.75, accuracy: 20 }, timestamp: Date.now() });
		};
	}`)
	participantPage.MustElement("button.button-refresh[onclick='getNearbyHikes()']").MustClick()
	assert.True(t, isElementVisible(t, participantPage, noOrgHikeListItemSelector, 10*time.Second), "NoOrg Hike should be in Nearby Hikes again")

	t.Logf("Participant RSVPing to NoOrg Hike Welcome (Join Code: %s)", joinCodeNoOrg)
	participantPage.MustElementX(noOrgHikeListItemSelector + "//button").MustClick() // Click RSVP for NoOrg Hike
	assert.True(t, isElementVisible(t, participantPage, "#join-hike-page", 5*time.Second), "Join page for NoOrg Hike")
	// Use same participant details or different, doesn't matter much for this test part
	participantPage.MustElement("#participant-name").MustInput("E2E WelcomeUser") // Can reuse form
	participantPage.MustElement("#participant-phone").MustInput("3030303030")
	participantPage.MustElement("#join-hike-form button[onclick='showWaiverPage()']").MustClick()
	assert.True(t, isElementVisible(t, participantPage, "#waiver-page", 5*time.Second), "Waiver page for NoOrg Hike")
	participantPage.MustElement("#waiver-page button[onclick='joinHike()']").MustClick()
	assert.True(t, isElementVisible(t, participantPage, "#welcome-page", 10*time.Second), "Welcome page after RSVP to NoOrg Hike")


	// --- Verify RSVPed Hikes List (No-Org Hike) ---
	t.Log("Verifying organization display in RSVPed Hikes list for No-Org Hike...")
	rsvpedNoOrgHikeListItemSelector := fmt.Sprintf("//ul[@id='rsvped-hikes-list']//li[contains(., 'NoOrg Hike Welcome') and .//button[contains(@onclick, \"startHiking('%s')\")]]", joinCodeNoOrg)
	assert.True(t, isElementVisible(t, participantPage, rsvpedNoOrgHikeListItemSelector, 5*time.Second), "NoOrg Hike should be in RSVPed Hikes")
	rsvpedNoOrgHikeElement := participantPage.MustElementX(rsvpedNoOrgHikeListItemSelector)
	assert.False(t, rsvpedNoOrgHikeElement.MustHas("p:contains('Organization:')"), "NoOrg Hike in RSVPed list should NOT display 'Organization:' text")
	t.Log("NoOrg Hike Welcome without organization found in RSVPed Hikes (and no org text displayed).")

	t.Log("TestOrganizationDisplayOnWelcomePage COMPLETED SUCCESSFULLY")
}

// Helper function to create a hike during E2E tests
func createHikeE2E(t *testing.T, page *rod.Page, hikeName, organization, trailheadQuery, trailheadFullName, leaderName, leaderPhone string, hoursFromNow int) {
	t.Helper()
	t.Logf("Creating hike: %s (Org: %s)", hikeName, organization)
	assert.True(t, isElementVisible(t, page, "button[onclick='showCreateHikePage()']", 10*time.Second), "Create New Hike button")
	page.MustElement("button[onclick='showCreateHikePage()']").MustClick()

	assert.True(t, isElementVisible(t, page, "#create-hike-page", 5*time.Second), "Create hike page")
	page.MustElement("#hike-name").MustInput(hikeName)
	if organization != "" {
		page.MustElement("#hike-organization").MustInput(organization)
	}
	page.MustElement("#hike-trailheadName").MustInput(trailheadQuery)
	// Using normalize-space(.) to match the full, cleaned-up text content of the div,
	// which should be robust against internal highlighting tags like <strong>.
	autocompleteSelector := fmt.Sprintf("//div[@class='autocomplete-items']/div[normalize-space(.)='%s']", trailheadFullName)
	assert.True(t, isElementVisible(t, page, autocompleteSelector, 7*time.Second), fmt.Sprintf("Autocomplete item for '%s' (query: '%s')", trailheadFullName, trailheadQuery))
	page.MustElementX(autocompleteSelector).MustClick()

	page.MustElement("#leader-name").MustInput(leaderName)
	page.MustElement("#leader-phone").MustInput(leaderPhone)

	hikeTime := time.Now().Add(time.Duration(hoursFromNow) * time.Hour)
	page.MustElement("input[placeholder='Click to select date and time'][type='text']").MustClick()
	assert.True(t, isElementVisible(t, page, ".flatpickr-calendar.open", 5*time.Second), "Flatpickr calendar")
	yearEl := page.MustElement(".flatpickr-current-month .numInput.cur-year")
	yearEl.MustSelectAllText().MustInput(fmt.Sprintf("%d", hikeTime.Year()))
	yearEl.MustType(input.Enter)
	daySelector := fmt.Sprintf(".flatpickr-day:not(.prevMonthDay):not(.nextMonthDay)[aria-label*='%s'][aria-label*='%d']", hikeTime.Format("January"), hikeTime.Day())
	assert.True(t, isElementVisible(t, page, daySelector, 5*time.Second), "Flatpickr day")
	page.MustElement(daySelector).MustClick()
	hourEl := page.MustElement(".flatpickr-time .numInput.flatpickr-hour")
	hourEl.MustSelectAllText().MustInput(fmt.Sprintf("%02d", hikeTime.Hour()))
	minuteEl := page.MustElement(".flatpickr-time .numInput.flatpickr-minute")
	minuteEl.MustSelectAllText().MustInput(fmt.Sprintf("%02d", hikeTime.Minute()))
	minuteEl.MustType(input.Enter)

	page.MustElement("#create-hike-form button[onclick='createHike()']").MustClick()
	assert.True(t, isElementVisible(t, page, "#hike-leader-page", 10*time.Second), "Hike leader page after creating "+hikeName)
	t.Logf("Hike '%s' created successfully.", hikeName)
}
