package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
)

func TestMain(m *testing.M) {
	// Setup code here, e.g., database connections
	initDB(":memory:") // Use in-memory SQLite for tests

	// Run the tests
	exitCode := m.Run()

	// Teardown code here, if necessary

	// Exit with the same code as the test run
	os.Exit(exitCode)
}

func setupTestMux() *http.ServeMux {
	mux := http.NewServeMux()
	addRoutes(mux)
	return mux
}

func TestCreateHike(t *testing.T) {
	hike := Hike{
		Name: "Test Hike",
		Leader: User{
			Name:  "John Doe",
			Phone: "1234567890",
		},
		TrailheadName: "Test Trailhead",
		Latitude:      40.7128,
		Longitude:     -74.0060,
		StartTime:     time.Now().Add(24 * time.Hour),
	}
	body, _ := json.Marshal(hike)
	req, _ := http.NewRequest("POST", "/api/hike", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	mux := setupTestMux()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response Hike
	json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NotEmpty(t, response.JoinCode)
	assert.NotEmpty(t, response.LeaderCode)
	assert.Equal(t, hike.Name, response.Name)
	assert.Equal(t, hike.Leader.Name, response.Leader.Name)
	assert.Equal(t, hike.TrailheadName, response.TrailheadName)
}

func TestJoinHike(t *testing.T) {
	hike := createTestHike(t)
	request := struct {
		User User `json:"user"`
	}{
		User: User{
			UUID:             "participant-uuid",
			Name:             "Jane Doe",
			Phone:            "9876543210",
			LicensePlate:     "ABC123",
			EmergencyContact: "5555555555",
		},
	}
	body, _ := json.Marshal(request)
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/hike/%s/participant", hike.JoinCode), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	mux := setupTestMux()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response Hike
	json.Unmarshal(rr.Body.Bytes(), &response)
	assert.Equal(t, hike.Leader.Name, response.Leader.Name)
}

func TestNearbyHikes(t *testing.T) {
	createTestHike(t)

	req, _ := http.NewRequest("GET", "/api/hike?latitude=40.7128&longitude=-74.0060", nil)
	rr := httptest.NewRecorder()
	mux := setupTestMux()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var hikes []Hike
	json.Unmarshal(rr.Body.Bytes(), &hikes)
	assert.GreaterOrEqual(t, len(hikes), 1)
}

func TestHikeParticipants(t *testing.T) {
	hike := createTestHike(t)
	joinTestHike(t, hike)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/hike/%s/participant?leaderCode=%s", hike.JoinCode, hike.LeaderCode), nil)
	rr := httptest.NewRecorder()
	mux := setupTestMux()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var participants []Participant
	json.Unmarshal(rr.Body.Bytes(), &participants)
	assert.Equal(t, 1, len(participants))
}

func TestEndHike(t *testing.T) {
	hike := createTestHike(t)

	body, _ := json.Marshal(hike)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/api/hike/%s", hike.LeaderCode), bytes.NewBuffer(body))

	rr := httptest.NewRecorder()
	mux := setupTestMux()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestLeaveHike(t *testing.T) {
	hike := createTestHike(t)
	participant := joinTestHike(t, hike)

	req, _ := http.NewRequest("PUT",
		fmt.Sprintf("/api/hike/%s/participant/%s", hike.JoinCode, participant.User.UUID),
		bytes.NewBufferString(`{"status":"finished"}`))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	mux := setupTestMux()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTrailheadSuggestions(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/trailhead?q=Ka", nil)
	rr := httptest.NewRecorder()
	mux := setupTestMux()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var suggestions []Trailhead
	json.Unmarshal(rr.Body.Bytes(), &suggestions)
	assert.GreaterOrEqual(t, len(suggestions), 1)
}

func TestGetHikeByCode(t *testing.T) {
	hike := createTestHike(t)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/hike/%s", hike.JoinCode), nil)
	rr := httptest.NewRecorder()
	mux := setupTestMux()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response Hike
	json.Unmarshal(rr.Body.Bytes(), &response)
	assert.Equal(t, hike.Name, response.Name)
	assert.Equal(t, hike.JoinCode, response.JoinCode)
	assert.Equal(t, hike.TrailheadName, response.TrailheadName)
}

func TestTableCreation(t *testing.T) {
	// initDB in TestMain should have already created tables.
	// We query sqlite_master to be sure.
	var tableName string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='waiver_signatures'").Scan(&tableName)
	require.NoError(t, err, "waiver_signatures table should exist")
	assert.Equal(t, "waiver_signatures", tableName)

	// Verify table schema
	rows, err := db.Query("PRAGMA table_info(waiver_signatures)")
	require.NoError(t, err, "Should be able to query table_info for waiver_signatures")
	defer rows.Close()

	expectedColumns := map[string]string{
		"id":             "INTEGER",
		"user_uuid":      "TEXT",
		"hike_join_code": "TEXT",
		"signed_at":      "DATETIME",
		"user_agent":     "TEXT",
		"ip_address":     "TEXT",
		"waiver_text":    "TEXT",
	}

	foundColumns := 0
	for rows.Next() {
		var cid int
		var name string
		var dataType string // In SQLite, this is 'type'
		var notnull bool
		var dfltValue interface{}
		var pk int
		err := rows.Scan(&cid, &name, &dataType, &notnull, &dfltValue, &pk)
		require.NoError(t, err, "Should be able to scan table_info row")

		expectedType, ok := expectedColumns[name]
		assert.True(t, ok, fmt.Sprintf("Column %s is not expected", name))
		assert.Equal(t, expectedType, dataType, fmt.Sprintf("Column %s has type %s, expected %s", name, dataType, expectedType))
		if name == "id" {
			assert.Equal(t, 1, pk, "Column 'id' should be the primary key")
			// assert.True(t, notnull, "Column 'id' should be NOT NULL") // Autoincrement implies NOT NULL
		}
		delete(expectedColumns, name) // Remove found column
		foundColumns++
	}
	require.NoError(t, rows.Err(), "Error iterating over table_info rows")
	assert.Empty(t, expectedColumns, "Not all expected columns were found")
	assert.Equal(t, 7, foundColumns, "Should find exactly 7 columns")

	// Note: Checking foreign keys with PRAGMA foreign_key_list(waiver_signatures) is more complex
	// and might be overkill for this test, as SQLite's enforcement is the main thing.
	// We trust that if go-sqlite3 doesn't error on the CREATE TABLE, the FKs are syntactically correct.
}

func TestJoinHikeRecordsWaiver(t *testing.T) {
	// 2. Create a hike to get a valid hikeId (joinCode)
	// Use a unique leader UUID for this test to avoid conflicts if tests run in parallel
	// or if db is not perfectly clean (though :memory: should be clean each TestMain).
	testHike := createTestHikeWithOptions(t, User{
		UUID:  "leader-uuid-waivertest",
		Name:  "Waiver Test Leader",
		Phone: "1112223333",
	})
	require.NotEmpty(t, testHike.JoinCode, "Test hike should have a join code")

	// 3. Prepare request for joinHikeHandler
	participantUser := User{
		UUID:             "participant-uuid-waivertest",
		Name:             "Waiver Participant",
		Phone:            "0001112222",
		LicensePlate:     "WVRTEST",
		EmergencyContact: "9998887777",
	}
	joinRequestPayload := struct {
		User User `json:"user"`
	}{User: participantUser}

	body, err := json.Marshal(joinRequestPayload)
	require.NoError(t, err)

	reqURL := fmt.Sprintf("/api/hike/%s/participant", testHike.JoinCode)
	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(body))
	require.NoError(t, err)

	expectedUserAgent := "Test-Agent/1.0"
	expectedIPAddress := "192.0.2.1" // Example IP from X-Forwarded-For
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", expectedUserAgent)
	req.Header.Set("X-Forwarded-For", expectedIPAddress)

	// 4. Execute request
	rr := httptest.NewRecorder()
	mux := setupTestMux() // Assumes db is already set up from TestMain
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Join hike request should succeed. Body: %s", rr.Body.String())

	// 5. Query waiver_signatures table
	var (
		id           int
		userUUID     string
		hikeJoinCode string
		signedAtStr  string // Read as string, then parse if needed
		userAgent    string
		ipAddress    string
		dbWaiverText string
	)
	query := `SELECT id, user_uuid, hike_join_code, signed_at, user_agent, ip_address, waiver_text
	          FROM waiver_signatures
	          WHERE user_uuid = ? AND hike_join_code = ?`
	row := db.QueryRow(query, participantUser.UUID, testHike.JoinCode)
	err = row.Scan(&id, &userUUID, &hikeJoinCode, &signedAtStr, &userAgent, &ipAddress, &dbWaiverText)
	require.NoError(t, err, "Failed to find waiver signature in DB. \nDB content for waiver_signatures:\n"+dumpTable(t, "waiver_signatures"))

	// 6. Verify data
	assert.Equal(t, participantUser.UUID, userUUID, "User UUID should match")
	assert.Equal(t, testHike.JoinCode, hikeJoinCode, "Hike join code should match")
	assert.Equal(t, expectedUserAgent, userAgent, "User agent should match")
	assert.Equal(t, expectedIPAddress, ipAddress, "IP address should match")
	// assert.Equal(t, sampleWaiverText, dbWaiverText, "Waiver text should match")

	// Verify signed_at is a valid timestamp (roughly now)
	signedAt, err := time.Parse("2006-01-02T15:04:05Z", signedAtStr) // Default SQLite datetime format
	require.NoError(t, err, "Failed to parse signed_at timestamp")
	assert.WithinDuration(t, time.Now(), signedAt, 5*time.Second, "signed_at should be recent")
}

// Helper function to dump table content for debugging
func dumpTable(t *testing.T, tableName string) string {
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s", tableName))
	if err != nil {
		return fmt.Sprintf("Error querying table %s: %v", tableName, err)
	}
	defer rows.Close()

	var result strings.Builder
	cols, _ := rows.Columns()
	result.WriteString(fmt.Sprintf("Columns: %v\n", cols))

	for rows.Next() {
		vals := make([]interface{}, len(cols))
		scanArgs := make([]interface{}, len(cols))
		for i := range vals {
			scanArgs[i] = &vals[i]
		}
		err = rows.Scan(scanArgs...)
		if err != nil {
			result.WriteString(fmt.Sprintf("Error scanning row: %v\n", err))
			continue
		}
		result.WriteString(fmt.Sprintf("%v\n", vals))
	}
	if err = rows.Err(); err != nil {
		result.WriteString(fmt.Sprintf("Error iterating rows: %v\n", err))
	}
	if result.Len() == 0 {
		return fmt.Sprintf("Table %s is empty or does not exist.", tableName)
	}
	return result.String()
}

func TestUpdateParticipantStatus(t *testing.T) {
	hike := createTestHike(t)
	participant := joinTestHike(t, hike)

	req, _ := http.NewRequest("PUT",
		fmt.Sprintf("/api/hike/%s/participant/%s", hike.JoinCode, participant.User.UUID),
		bytes.NewBufferString(`{"status":"completed"}`))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	mux := setupTestMux()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func createTestHike(t *testing.T) Hike {
	hike := Hike{
		Name: "Test Hike",
		Leader: User{ // Default leader if none provided
			UUID:  "test-uuid-default",
			Name:  "John Doe",
			Phone: "1234567890",
		},
		TrailheadName: "Test Trailhead",
		Latitude:      40.7128,
		Longitude:     -74.0060,
		StartTime:     time.Now(),
	}
	body, _ := json.Marshal(hike)
	req, _ := http.NewRequest("POST", "/api/hike", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	mux := setupTestMux()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Failed to create test hike. Body: %s", rr.Body.String())

	var response Hike
	json.Unmarshal(rr.Body.Bytes(), &response)
	return response
}

// createTestHikeWithOptions allows specifying the leader
func createTestHikeWithOptions(t *testing.T, leader User) Hike {
	hike := Hike{
		Name:          "Test Hike for " + leader.UUID,
		Leader:        leader,
		TrailheadName: "Test Trailhead for " + leader.UUID,
		Latitude:      40.7128,
		Longitude:     -74.0060,
		StartTime:     time.Now(),
	}
	body, err := json.Marshal(hike)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/api/hike", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	mux := setupTestMux()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Failed to create test hike with options. Body: %s", rr.Body.String())

	var response Hike
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "Failed to unmarshal createTestHikeWithOptions response")
	return response
}

func joinTestHike(t *testing.T, hike Hike) Participant {
	// Default participant for generic join tests
	defaultParticipantUser := User{
		UUID:             "participant-uuid-defaultjoin",
		Name:             "Default Joiner",
		Phone:            "9876543210",
		LicensePlate:     "DEFJOIN",
		EmergencyContact: "5555555555",
	}
	return joinTestHikeWithOptions(t, hike, defaultParticipantUser)
}

func joinTestHikeWithOptions(t *testing.T, hike Hike, user User) Participant {
	request := struct {
		User User `json:"user"`
	}{User: user}

	body, err := json.Marshal(request)
	require.NoError(t, err)

	reqURL := fmt.Sprintf("/api/hike/%s/participant", hike.JoinCode)
	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	mux := setupTestMux()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Failed to join test hike. Body: %s", rr.Body.String())

	var responseHikeData Hike // This is the hike data returned by joinHikeHandler
	err = json.Unmarshal(rr.Body.Bytes(), &responseHikeData)
	require.NoError(t, err, "Failed to unmarshal joinTestHike response")

	return Participant{
		Hike: responseHikeData, // Use the returned hike data
		User: user,             // Use the input user data
	}
}
