package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
)

// Keep in sync with trailheads table schema
type Trailhead struct {
	Name    string `json:"name"`
	MapLink string `json:"mapLink"`
}

// List of predefined trailheads
var predefinedTrailheads = []Trailhead{
	{Name: "Aiea Loop (upper)", MapLink: "https://maps.app.goo.gl/cAFXUQF6Gbk1Yx9k6"},
	{Name: "Bowman (Radar Hill)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.34992,-157.87685"},
	{Name: "Crouching Lion (Manamana)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.55816,-157.86619"},
	{Name: "Diamond Head Crater (Le'ahi)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.26360,-157.80603"},
	{Name: "Ehukai Pillbox (Sunset Pillbox)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.66465,-158.04936"},
	{Name: "Friendship Garden", MapLink: "https://maps.app.goo.gl/wtBZ6b2YyEkVeZpq5"},
	{Name: "Haha'ione", MapLink: "https://www.google.com/maps/search/?api=1&query=21.310139,-157.712835"},
	{Name: "Hamana Falls", MapLink: "https://www.google.com/maps/search/?api=1&query=21.45293,-157.85281"},
	{Name: "Hau'ula Loop", MapLink: "https://www.google.com/maps/search/?api=1&query=21.60980,-157.91544"},
	{Name: "Hawaii Loa Ridge", MapLink: "https://maps.app.goo.gl/xLvemfsw6oVz6La16"},
	{Name: "Ho'omaluhia Botanical Garden", MapLink: "https://maps.app.goo.gl/ASYuQiyFyJ1NVf5u7"},
	{Name: "Judd", MapLink: "https://maps.app.goo.gl/BqeFRzzjMC7LdK2v9"},
	{Name: "Ka'au Crater", MapLink: "https://maps.app.goo.gl/mHuxvn71uiARPmbR8"},
	{Name: "Ka'ena Point (Mokule'ia Side)", MapLink: "https://maps.app.goo.gl/gCBj3NgnG9yk2teSA"},
	{Name: "Ka'ena Point (Waianae Side)", MapLink: "https://maps.app.goo.gl/6CPM2UMuPQFxiTa59"},
	{Name: "Kahana Valley", MapLink: "https://www.google.com/maps/search/?api=1&query=21.55023,-157.88163"},
	{Name: "Kahekili Ridge", MapLink: "https://www.google.com/maps/search/?api=1&query=21.55410,-157.85579"},
	{Name: "Kaipapa'u Gulch", MapLink: "https://www.google.com/maps/search/?api=1&query=21.61809,-157.91893"},
	{Name: "Ka'iwa Ridge (Lanikai Side)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.39031,-157.71943"},
	{Name: "Ka'iwa Ridge (Keolu Side)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.38174,-157.72553"},
	{Name: "Kalawahine", MapLink: "https://www.google.com/maps/search/?api=1&query=21.33125,-157.82128"},
	{Name: "Kamana'iki", MapLink: "https://www.google.com/maps/search/?api=1&query=21.34960,-157.85821"},
	{Name: "Kamilo'iki", MapLink: "https://www.google.com/maps/search/?api=1&query=21.300515,-157.692755"},
	{Name: "Kaniakapupu Ruins", MapLink: "https://www.google.com/maps/search/?api=1&query=21.351083,-157.81698"},
	{Name: "Kapa'ele'ele", MapLink: "https://www.google.com/maps/search/?api=1&query=21.55501,-157.87682"},
	{Name: "Kapena Falls", MapLink: "https://www.google.com/maps/search/?api=1&query=21.32401,-157.84699"},
	{Name: "Kaunala", MapLink: "https://www.google.com/maps/search/?api=1&query=21.64290,-158.02590"},
	{Name: "Kealia", MapLink: "https://www.google.com/maps/search/?api=1&query=21.57750,-158.20816"},
	{Name: "Kea'au Middle Ridge", MapLink: "https://www.google.com/maps/search/?api=1&query=21.50296,-158.22544"},
	{Name: "Koko Crater (Arch)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.28069,-157.67854"},
	{Name: "Koko Crater (Railway)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.28117,-157.69192"},
	{Name: "Koko Head (Hanauma)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.27532,-157.69363"},
	{Name: "Koloa Gulch", MapLink: "https://www.google.com/maps/search/?api=1&query=21.62817,-157.923531"},
	{Name: "Kuliʻouʻou Ridge", MapLink: "https://www.google.com/maps/search/?api=1&query=21.30343,-157.72426"},
	{Name: "Kulepeamoa Ridge", MapLink: "https://www.google.com/maps/search/?api=1&query=21.29218,-157.74093"},
	{Name: "Laie Falls (parking)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.65053,-157.93147"},
	{Name: "Lanihuli", MapLink: "https://www.google.com/maps/search/?api=1&query=21.33986,-157.84751"},
	{Name: "Lanipo", MapLink: "https://www.google.com/maps/search/?api=1&query=21.29787,-157.78574"},
	{Name: "Likeke Falls (First Pres)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.37281,-157.79209"},
	{Name: "Lulumahu Falls", MapLink: "https://www.google.com/maps/search/?api=1&query=21.354438,-157.81114"},
	{Name: "Makapu'u Point Lighthouse", MapLink: "https://www.google.com/maps/search/?api=1&query=21.30499,-157.65480"},
	{Name: "Makiki Valley Loop (Nature Center)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.31717,-157.82700"},
	{Name: "Manana Ridge", MapLink: "https://www.google.com/maps/search/?api=1&query=21.43038,-157.93889"},
	{Name: "Manoa Cliff", MapLink: "https://www.google.com/maps/search/?api=1&query=21.32612,-157.81308"},
	{Name: "Manoa Falls", MapLink: "https://www.google.com/maps/search/?api=1&query=21.33255,-157.80055"},
	{Name: "Maunawili Falls", MapLink: "https://www.google.com/maps/search/?api=1&query=21.35929,-157.76355"},
	{Name: "Maunawili Demonstration (Pali)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.36496,-157.77998"},
	{Name: "Maunawili Ditch (Wakupanaha)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.34294,-157.74341"},
	{Name: "Maunawili Ditch (Mahiku)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.34918,-157.73400"},
	{Name: "Moanalua Valley", MapLink: "https://www.google.com/maps/search/?api=1&query=21.37412,-157.88061"},
	{Name: "Mount Ka'ala", MapLink: "https://www.google.com/maps/search/?api=1&query=21.47597,-158.15193"},
	{Name: "Nahuina", MapLink: "https://www.google.com/maps/search/?api=1&query=21.32978,-158.82265"},
	{Name: "Ohana Bike (N)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.37203,-157.74520"},
	{Name: "Ohana Bike (S)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.35772,-157.73318"},
	{Name: "Olomana", MapLink: "https://www.google.com/maps/search/?api=1&query=21.36845,-157.76097"},
	{Name: "Pali Notches", MapLink: "https://www.google.com/maps/search/?api=1&query=21.36670,-157.79322"},
	{Name: "Pali Puka", MapLink: "https://www.google.com/maps/search/?api=1&query=21.36682,-157.79417"},
	{Name: "Puʻu Māʻeliʻel", MapLink: "https://www.google.com/maps/search/?api=1&query=21.43429,-157.82463"},
	{Name: "Pu'u Manamana", MapLink: "https://www.google.com/maps/search/?api=1&query=21.55410,-157.85579"},
	{Name: "Pu'u Ohia", MapLink: "https://www.google.com/maps/search/?api=1&query=21.33109,-157.81465"},
	{Name: "Pu'u O Hulu (Pink Pillbox)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.40478,-158.17268"},
	{Name: "Pu'u Pia Trail", MapLink: "https://www.google.com/maps/search/?api=1&query=21.32168,-157.79873"},
	{Name: "Tantalus Arboretum", MapLink: "https://www.google.com/maps/search/?api=1&query=21.32582,-157.82771"},
	{Name: "Tom Tom", MapLink: "https://www.google.com/maps/search/?api=1&query=21.32499,-157.69683"},
	{Name: "Ualakaa", MapLink: "https://www.google.com/maps/search/?api=1&query=21.31645,-157.82037"},
	{Name: "Wa'ahila Ridge", MapLink: "https://www.google.com/maps/search/?api=1&query=21.30729,-157.79765"},
	{Name: "Wahiawa Hills", MapLink: "https://www.google.com/maps/search/?api=1&query=21.50846,-157.98648"},
	{Name: "Waiau (parking)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.41257,-157.93985"},
	{Name: "Wailupe Valley (Hao)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.29861,-157.75663"},
	{Name: "Wailupe Valley (Mona)", MapLink: "https://www.google.com/maps/search/?api=1&query=21.29999,-157.75466"},
	{Name: "Waimalu Ditch", MapLink: "https://www.google.com/maps/search/?api=1&query=21.39888,-157.91763"},
	{Name: "Waimano Ridge", MapLink: "https://www.google.com/maps/search/?api=1&query=21.41725,-157.95104"},
	{Name: "Waipuilani Falls", MapLink: "https://www.google.com/maps/search/?api=1&query=21.3643,-157.7959"},
	{Name: "Waipuhia Falls", MapLink: "https://www.google.com/maps/search/?api=1&query=21.36173,-157.80544"},
	{Name: "Wiliwilinui Ridge", MapLink: "https://www.google.com/maps/search/?api=1&query=21.29927,-157.76274"},
}

type User struct {
	UUID             string `json:"uuid"`
	Name             string `json:"name"`
	Phone            string `json:"phone"`
	LicensePlate     string `json:"licensePlate"`
	EmergencyContact string `json:"emergencyContact"`
}

// Keep in sync with hikes table schema
type Hike struct {
	ParticipantId       int64     `json:"participantId"` // Used when returning hike to User not in table
	Name                string    `json:"name"`          // Custom name for the hike event
	Organization        string    `json:"organization"`
	TrailheadName       string    `json:"trailheadName"`
	Leader              User      `json:"leader"`
	TrailheadMapLink    string    `json:"trailheadMapLink,omitempty"`
	CreatedAt           time.Time `json:"-"` // don't send this field in JSON response
	StartTime           time.Time `json:"startTime"`
	Status              string    `json:"Status"`
	JoinCode            string    `json:"joinCode"`
	LeaderCode          string    `json:"leaderCode"`
	PhotoRelease        bool      `json:"photoRelease"`
	SourceType          string    `json:"sourceType,omitempty"` // Added for combined hike results
	DescriptionMarkdown string    `json:"descriptionMarkdown"`
	DescriptionHTML     string    `json:"descriptionHTML"`
	WaiverText          string    `json:"waiverText,omitempty"`
}

// Keep in sync with participants table schema
type Participant struct {
	Id       int64     `json:"id"`
	Hike     Hike      `json:"hike"`
	User     User      `json:"user"`
	Status   string    `json:"status"`
	Waiver   time.Time `json:"waiver"`
	JoinedAt time.Time `json:"joinedAt"`
}

var db *sql.DB

func createTables() {
	// Create tables if they don't exist
	// Note to self: Foreign key declarations must be at the end of the table creation statement
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS trailheads (
			name TEXT PRIMARY KEY,
			map_link TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS users (
			uuid TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			phone TEXT,
			license_plate TEXT,
			emergency_contact TEXT
		);

		CREATE TABLE IF NOT EXISTS hikes (
			name TEXT NOT NULL,
            organization TEXT,
			trailhead_name TEXT,
			leader_uuid TEXT NOT NULL,
			trailhead_map_link TEXT,
			created_at DATETIME NOT NULL,
			start_time DATETIME NOT NULL,
			status TEXT DEFAULT 'open',
			join_code TEXT PRIMARY KEY,
			leader_code TEXT UNIQUE,
            photo_release BOOLEAN DEFAULT FALSE,
            description TEXT,
			FOREIGN KEY (leader_uuid) REFERENCES users(uuid)
		);

		CREATE TABLE IF NOT EXISTS hike_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hike_join_code TEXT,
			user_uuid TEXT,
			status TEXT DEFAULT 'active',
			joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            UNIQUE (hike_join_code, user_uuid),
			FOREIGN KEY (hike_join_code) REFERENCES hikes(join_code),
			FOREIGN KEY (user_uuid) REFERENCES users(uuid)
		);

		CREATE TABLE IF NOT EXISTS waiver_signatures (
			user_uuid TEXT,
			hike_join_code TEXT,
			signed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			user_agent TEXT NOT NULL,
			ip_address TEXT NOT NULL,
			waiver_text TEXT NOT NULL,
            PRIMARY KEY (user_uuid, hike_join_code),
			FOREIGN KEY (user_uuid) REFERENCES users(uuid),
			FOREIGN KEY (hike_join_code) REFERENCES hikes(join_code)
		);
	`)
	if err != nil {
		log.Fatal(err)
	}
}

func populateTrailheads() {
	// Load trailheads if they haven't been loaded yet
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM trailheads").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	if count == 0 {
		for _, trailhead := range predefinedTrailheads {
			_, err := db.Exec("INSERT INTO trailheads (name, map_link) VALUES (?, ?)", trailhead.Name, trailhead.MapLink)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

// Database initialization should be in its own function so that it can be called from tests rather than in init()
func initDB(databaseName string) {
	var err error
	db, err = sql.Open("sqlite3", databaseName)
	if err != nil {
		log.Fatal(err)
	}
	createTables()
	populateTrailheads()
}

// Add routes to ServeMux (sparate function so it can be used in testing)
func addRoutes(mux *http.ServeMux) {
	// You must define most specific routes first
	mux.HandleFunc("PUT /api/hike/{hikeId}/participant/{participantId}", updateParticipantStatusHandler)
	mux.HandleFunc("POST /api/hike/{hikeId}/participant", rsvpToHikeHandler) // pass in User
	mux.HandleFunc("DELETE /api/hike/{hikeId}/participant/{participantId}", unRSVPHandler)
	mux.HandleFunc("GET /api/hike/{hikeId}/participant", getHikeParticipantsHandler)
	// mux.HandleFunc("GET /api/hike/{hikeId}/waiver", getHikeWaiverHandler) // Removed
	mux.HandleFunc("GET /api/hike/{hikeId}", getHikeHandler)
	mux.HandleFunc("PUT /api/hike/{hikeId}", endHikeHandler) // require leader code
	mux.HandleFunc("POST /api/hike", createHikeHandler)
	mux.HandleFunc("GET /api/hike/last", getLastHikeHandler) // Return the last hike details for a given hikeName and leaderUUID
	mux.HandleFunc("GET /api/hike", getHikesHandler)
	mux.HandleFunc("GET /api/trailhead", trailheadSuggestionsHandler)
	// GET /api/userhikes/{userUUID} is now handled by GET /api/hike?userUUID=...
	mux.HandleFunc("PUT /api/hike/update/{leaderCode}", updateHikeHandler) // Path for updating hike details
}

// updateHikeHandler processes requests to update hike details.
// It uses the leaderCode from the path for authorization and identification.
func updateHikeHandler(w http.ResponseWriter, r *http.Request) {
	leaderCode := r.PathValue("leaderCode")
	if leaderCode == "" {
		http.Error(w, "Leader code is required", http.StatusBadRequest)
		return
	}

	var updates Hike // Using the existing Hike struct for simplicity, assuming partial updates
	err := json.NewDecoder(r.Body).Decode(&updates)
	if err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Failed to start transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback() // Rollback if not committed

	// Fetch current leader_uuid to see if it's changing
	var currentLeaderUUID string
	var currentJoinCode string // Need join_code to regenerate waiver if leader/org changes
	err = tx.QueryRow("SELECT leader_uuid, join_code FROM hikes WHERE leader_code = ?", leaderCode).Scan(&currentLeaderUUID, &currentJoinCode)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Hike not found for the given leader code", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching current hike details: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// If a new leader UUID is provided and it's different from the current one
	newLeaderProvided := updates.Leader.UUID != "" && updates.Leader.UUID != currentLeaderUUID
	if newLeaderProvided {
		// Ensure the new leader exists in the users table, or insert/update them
		if updates.Leader.Name != "" || updates.Leader.Phone != "" {
			_, err = tx.Exec(`
				INSERT INTO users (uuid, name, phone) VALUES (?, ?, ?)
				ON CONFLICT(uuid) DO UPDATE SET
					name = COALESCE(excluded.name, users.name),
					phone = COALESCE(excluded.phone, users.phone)
			`, updates.Leader.UUID, updates.Leader.Name, updates.Leader.Phone)
		} else {
			// If only UUID is provided, ensure the user exists, insert if not (with blank name/phone).
			_, err = tx.Exec(`INSERT OR IGNORE INTO users (uuid, name, phone) VALUES (?, '', '')`, updates.Leader.UUID)
		}
		if err != nil {
			http.Error(w, "Failed to update leader information in users table: "+err.Error(), http.StatusInternalServerError)
			return
		}
		currentLeaderUUID = updates.Leader.UUID // Update currentLeaderUUID for the SET clause below
	}


	var setClauses []string
	var args []interface{}
	anyFieldUpdated := false // Track if any updatable field (other than leader) was actually provided

	// Check if fields are provided and add them to the update query
	// For robustness, ideally the 'updates' struct would use pointers for optional fields
	// to distinguish between a field not provided and a field provided with its zero value.
	// Given the current struct, we assume non-empty/non-zero means an update is intended.

	if updates.Name != "" { // Simpler check: if name is non-empty, update it.
		setClauses = append(setClauses, "name = ?")
		args = append(args, updates.Name)
		anyFieldUpdated = true
	}

	// To properly handle partial updates where a field might be intentionally set to an empty string
	// or a boolean to false, we need to know if the field was present in the request.
	// A common way is to unmarshal into a map[string]json.RawMessage to check key presence.
	requestBodyBytes, err := json.Marshal(updates) // This is inefficient, ideally read r.Body once
	if err != nil {
		http.Error(w, "Could not re-marshal request body for field checking", http.StatusInternalServerError)
		return
	}
	var tempMap map[string]json.RawMessage
	err = json.Unmarshal(requestBodyBytes, &tempMap)
	if err != nil {
		http.Error(w, "Could not unmarshal to temp map for field checking", http.StatusInternalServerError)
		return
	}

	if _, ok := tempMap["organization"]; ok {
		setClauses = append(setClauses, "organization = ?")
		args = append(args, updates.Organization) // updates.Organization will be "" if key present but value empty
		anyFieldUpdated = true
	}
	if _, ok := tempMap["trailheadName"]; ok {
		// Only update if non-empty, assuming empty means "no change" for this specific field if key is present.
		// This is a business logic choice. If empty string is a valid clear, then just `updates.TrailheadName != ""` is not needed.
		if updates.TrailheadName != "" {
			setClauses = append(setClauses, "trailhead_name = ?")
			args = append(args, updates.TrailheadName)
			anyFieldUpdated = true
		} else if updates.TrailheadName == "" && string(tempMap["trailheadName"]) == `""` { // Explicitly set to empty string
            setClauses = append(setClauses, "trailhead_name = ?")
            args = append(args, "")
            anyFieldUpdated = true
        }
	}
	if _, ok := tempMap["trailheadMapLink"]; ok {
		setClauses = append(setClauses, "trailhead_map_link = ?")
		args = append(args, updates.TrailheadMapLink) // Allow empty string to clear
		anyFieldUpdated = true
	}
	if _, ok := tempMap["startTime"]; ok && !updates.StartTime.IsZero() {
		setClauses = append(setClauses, "start_time = ?")
		args = append(args, updates.StartTime.Format("2006-01-02T15:04:05-07:00"))
		anyFieldUpdated = true
	}
	if _, ok := tempMap["photoRelease"]; ok {
		setClauses = append(setClauses, "photo_release = ?")
		args = append(args, updates.PhotoRelease)
		anyFieldUpdated = true
	}
	if _, ok := tempMap["descriptionMarkdown"]; ok {
		setClauses = append(setClauses, "description = ?")
		args = append(args, updates.DescriptionMarkdown) // Allow empty string to clear
		anyFieldUpdated = true
	}

	// If new leader was provided, leader_uuid must be updated.
	if newLeaderProvided {
		// Avoid adding duplicate "leader_uuid = ?" if it's already part of a more complex update logic
		alreadyUpdatingLeader := false
		for _, clause := range setClauses {
			if strings.HasPrefix(clause, "leader_uuid =") {
				alreadyUpdatingLeader = true
				break
			}
		}
		if !alreadyUpdatingLeader {
			setClauses = append(setClauses, "leader_uuid = ?")
			args = append(args, currentLeaderUUID) // This is now the new leader's UUID
		}
	}


	if !anyFieldUpdated && !newLeaderProvided {
		// No fields to update were provided. Fetch and return current state.
	} else if len(setClauses) > 0 { // Only run DB update if there's something to update
		query := fmt.Sprintf("UPDATE hikes SET %s WHERE leader_code = ?", strings.Join(setClauses, ", "))
		args = append(args, leaderCode)

		result, err := tx.Exec(query, args...)
		if err != nil {
			http.Error(w, "Failed to update hike: "+err.Error(), http.StatusInternalServerError)
			return
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			http.Error(w, "Error checking affected rows: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			// log.Printf("Update for leader_code %s affected 0 rows.", leaderCode)
		}
	}

	err = tx.Commit()
	if err != nil {
		http.Error(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch the possibly updated hike to return it
	var updatedHike Hike
	var descriptionMarkdown sql.NullString
	var trailheadMapLink sql.NullString
	var org sql.NullString

	err = db.QueryRow(`
		SELECT h.name, h.organization, h.trailhead_name, u.uuid, u.name, u.phone,
		       h.trailhead_map_link, h.start_time, h.status, h.join_code, h.leader_code,
		       h.photo_release, h.description
		FROM hikes h
		JOIN users u ON h.leader_uuid = u.uuid
		WHERE h.leader_code = ?
	`, leaderCode).Scan(
		&updatedHike.Name, &org, &updatedHike.TrailheadName,
		&updatedHike.Leader.UUID, &updatedHike.Leader.Name, &updatedHike.Leader.Phone,
		&trailheadMapLink, &updatedHike.StartTime, &updatedHike.Status,
		&updatedHike.JoinCode, &updatedHike.LeaderCode, &updatedHike.PhotoRelease, &descriptionMarkdown,
	)

	if err != nil {
		http.Error(w, "Failed to retrieve full updated hike details: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if org.Valid {
		updatedHike.Organization = org.String
	} else {
		updatedHike.Organization = "" // Ensure it's empty string if DB NULL
	}

	if descriptionMarkdown.Valid {
		updatedHike.DescriptionMarkdown = descriptionMarkdown.String
		var buf strings.Builder
		if errMd := goldmark.Convert([]byte(updatedHike.DescriptionMarkdown), &buf); errMd != nil {
			log.Printf("Error converting description to HTML for updated hike %s: %v", updatedHike.JoinCode, errMd)
			updatedHike.DescriptionHTML = updatedHike.DescriptionMarkdown // Fallback
		} else {
			p := bluemonday.UGCPolicy()
			updatedHike.DescriptionHTML = p.Sanitize(buf.String())
		}
	} else {
		updatedHike.DescriptionMarkdown = ""
		updatedHike.DescriptionHTML = ""
	}

	if trailheadMapLink.Valid {
		updatedHike.TrailheadMapLink = trailheadMapLink.String
	} else {
		updatedHike.TrailheadMapLink = "" // Ensure it's empty string if DB NULL
	}

	waiverText, errWaiver := generateWaiverText(currentJoinCode)
	if errWaiver != nil {
		log.Printf("Error generating waiver text for updated hike %s (join_code %s): %v", updatedHike.LeaderCode, currentJoinCode, errWaiver)
		updatedHike.WaiverText = ""
	} else {
		updatedHike.WaiverText = waiverText
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedHike)
	logAction(fmt.Sprintf("Hike updated via leader code: %s", leaderCode))
}


func main() {
	initDB("./hiketracker.db")

	addRoutes(http.DefaultServeMux)

	// Serve static files
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	log.Println("Server starting on :8196")
	log.Fatal(http.ListenAndServe(":8196", nil))
}

// WaiverData is used to populate the waiver template
type WaiverData struct {
	LeaderName   string
	Organization string
	PhotoRelease bool
}

// generateWaiverText fetches hike details and generates the waiver text using a template.
func generateWaiverText(joinCode string) (string, error) {
	var leaderName, organization sql.NullString // Use sql.NullString for organization as it can be NULL
	var photoRelease bool

	err := db.QueryRow(`
		SELECT u.name, h.organization, h.photo_release
		FROM hikes h
		JOIN users u ON h.leader_uuid = u.uuid
		WHERE h.join_code = ?
	`, joinCode).Scan(&leaderName, &organization, &photoRelease)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("hike not found for join code: %s", joinCode)
		}
		return "", fmt.Errorf("error fetching hike details for waiver: %v", err)
	}

	data := WaiverData{
		LeaderName:   leaderName.String,
		Organization: organization.String,
		PhotoRelease: photoRelease,
	}

	// Read waiver template
	templateBytes, err := os.ReadFile("static/waiver.txt")
	if err != nil {
		return "", fmt.Errorf("error reading waiver.txt template: %v", err)
	}
	templateContent := string(templateBytes)

	// Parse and execute template
	tmpl, err := template.New("waiver").Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("error parsing waiver template: %v", err)
	}

	var renderedWaiver strings.Builder
	if err := tmpl.Execute(&renderedWaiver, data); err != nil {
		return "", fmt.Errorf("error executing waiver template: %v", err)
	}

	return renderedWaiver.String(), nil
}

// getHikeWaiverHandler is removed. Waiver text is now part of Hike object.

func getLastHikeHandler(w http.ResponseWriter, r *http.Request) {
	hikeName := r.URL.Query().Get("hikeName")
	leaderUUID := r.URL.Query().Get("leaderUUID")

	if hikeName == "" || leaderUUID == "" {
		http.Error(w, "hikeName and leaderUUID query parameters are required", http.StatusBadRequest)
		return
	}

	var hike Hike
	var descriptionMarkdown sql.NullString
	err := db.QueryRow(`
	    SELECT name, organization, trailhead_name, trailhead_map_link, description
		FROM hikes
		WHERE name = ? AND leader_uuid = ?
		ORDER BY created_at DESC, rowid DESC
		LIMIT 1
	`, hikeName, leaderUUID).Scan(&hike.Name, &hike.Organization, &hike.TrailheadName, &hike.TrailheadMapLink, &descriptionMarkdown)

	if err != nil {
		if err == sql.ErrNoRows {
			// No hike found, return empty JSON or appropriate error
			w.WriteHeader(http.StatusNotFound) // Or return an empty Hike object
			json.NewEncoder(w).Encode(Hike{})   // Return empty hike
			return
		}
		// For other errors, return internal server error
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if descriptionMarkdown.Valid {
		hike.DescriptionMarkdown = descriptionMarkdown.String
		// Convert markdown to HTML
		var buf strings.Builder
		if err := goldmark.Convert([]byte(hike.DescriptionMarkdown), &buf); err != nil {
			log.Printf("Error converting description to HTML for last hike %s: %v", hike.Name, err)
			// Send raw markdown in HTML field if conversion fails, or leave HTML field empty
			hike.DescriptionHTML = hike.DescriptionMarkdown // Or consider leaving empty: hike.DescriptionHTML = ""
		} else {
			p := bluemonday.UGCPolicy()
			hike.DescriptionHTML = p.Sanitize(buf.String())
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hike)
}

// Create a new hike and return codes for leader and participants to access the hike
func createHikeHandler(w http.ResponseWriter, r *http.Request) {

	// Extract json Hike
	var hike Hike
	err := json.NewDecoder(r.Body).Decode(&hike)
	// fmt.Printf("%+v\n", hike)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate secure code to use for participants to join the hike
	hike.JoinCode, err = generateSecureLinkCode()
	if err != nil {
		http.Error(w, "Failed to generate join code", http.StatusInternalServerError)
		return
	}

	// Generate secure code to use for the hike leader to manage the hike
	hike.LeaderCode, err = generateSecureLinkCode()
	if err != nil {
		http.Error(w, "Failed to generate leader code", http.StatusInternalServerError)
		return
	}

	hike.CreatedAt = time.Now()

	// Insert or update user (Leader) in the database
	_, err = db.Exec(`
		INSERT INTO users (uuid, name, phone)
		VALUES (?, ?, ?)
		ON CONFLICT(uuid) DO UPDATE SET name = excluded.name, phone = excluded.phone
		`, hike.Leader.UUID, hike.Leader.Name, hike.Leader.Phone)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Add hike to the Hikes table
	// Note: hike.DescriptionMarkdown contains the raw markdown from the request
	_, err = db.Exec(`
		INSERT INTO hikes (name, organization, trailhead_name, leader_uuid, trailhead_map_link, created_at, start_time, join_code, leader_code, photo_release, description)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, hike.Name, hike.Organization, hike.TrailheadName, hike.Leader.UUID, hike.TrailheadMapLink, hike.CreatedAt.Format("2006-01-02T15:04:05-07:00"), hike.StartTime, hike.JoinCode, hike.LeaderCode, hike.PhotoRelease, hike.DescriptionMarkdown)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Populate DescriptionHTML for the response
	if hike.DescriptionMarkdown != "" {
		var buf strings.Builder
		if err := goldmark.Convert([]byte(hike.DescriptionMarkdown), &buf); err != nil {
			log.Printf("Error converting description to HTML for new hike %s: %v", hike.JoinCode, err)
			// In case of error, DescriptionHTML might be empty or fallback to markdown
			hike.DescriptionHTML = hike.DescriptionMarkdown
		} else {
			p := bluemonday.UGCPolicy()
			hike.DescriptionHTML = p.Sanitize(buf.String())
		}
	} else {
		hike.DescriptionHTML = ""
	}

	// Generate and add waiver text
	waiverText, err := generateWaiverText(hike.JoinCode)
	if err != nil {
		// Log the error, but don't fail the request. The client can choose how to handle missing waiver text.
		log.Printf("Error generating waiver text for new hike %s: %v", hike.JoinCode, err)
		hike.WaiverText = "" // Set to empty or some error message if preferred
	} else {
		hike.WaiverText = waiverText
	}

	json.NewEncoder(w).Encode(hike)
	logAction(fmt.Sprintf("Hike created: %s by %s, starting at %s", hike.Name, hike.Leader.Name, hike.StartTime.Format(time.RFC3339)))
}

// Get hike details by join code, Don't return leader code
func getHikeHandler(w http.ResponseWriter, r *http.Request) {
	joinCode := r.PathValue("hikeId")
	leaderCode := r.URL.Query().Get("leaderCode")

	// Retrieve Hike record based on leaderCode if provided otherwise by joinCode
	var hike Hike
	var descriptionMarkdown sql.NullString // Use sql.NullString for description as it can be NULL
	var trailheadMapLink sql.NullString
	var err error
	if leaderCode != "" {
		err = db.QueryRow(`SELECT h.name, h.organization, h.trailhead_name, u.name, u.phone, h.trailhead_map_link, h.start_time, h.join_code, h.description
		                   FROM hikes As h JOIN users AS u ON leader_uuid = uuid
		                   WHERE h.leader_code = ? AND h.status = "open"
		`, leaderCode).Scan(&hike.Name, &hike.Organization, &hike.TrailheadName, &hike.Leader.Name, &hike.Leader.Phone, &trailheadMapLink, &hike.StartTime, &hike.JoinCode, &descriptionMarkdown)
	} else {
		err = db.QueryRow(`SELECT h.name, h.organization, h.trailhead_name, u.name, u.phone, h.trailhead_map_link, h.start_time, h.join_code, h.description
						   FROM hikes As h JOIN users AS u ON leader_uuid = uuid
						   WHERE h.join_code = ? AND h.status = "open"
		`, joinCode).Scan(&hike.Name, &hike.Organization, &hike.TrailheadName, &hike.Leader.Name, &hike.Leader.Phone, &trailheadMapLink, &hike.StartTime, &hike.JoinCode, &descriptionMarkdown)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Hike not found or already closed", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if descriptionMarkdown.Valid {
		hike.DescriptionMarkdown = descriptionMarkdown.String
		// Convert markdown to HTML
		var buf strings.Builder
		if err := goldmark.Convert([]byte(hike.DescriptionMarkdown), &buf); err != nil {
			log.Printf("Error converting description to HTML for hike %s: %v", hike.JoinCode, err)
			hike.DescriptionHTML = hike.DescriptionMarkdown // Fallback or leave empty
		} else {
			p := bluemonday.UGCPolicy()
			hike.DescriptionHTML = p.Sanitize(buf.String())
		}
	} else {
		hike.DescriptionMarkdown = ""
		hike.DescriptionHTML = ""
	}

	if trailheadMapLink.Valid {
		hike.TrailheadMapLink = trailheadMapLink.String
	}

	// Generate and add waiver text
	waiverText, err := generateWaiverText(hike.JoinCode)
	if err != nil {
		log.Printf("Error generating waiver text for get hike %s: %v", hike.JoinCode, err)
		hike.WaiverText = ""
	} else {
		hike.WaiverText = waiverText
	}

	// Return retrieved Hike
	json.NewEncoder(w).Encode(hike)
}

// End a hike and mark all participants as finished
func endHikeHandler(w http.ResponseWriter, r *http.Request) {
	joinCode := r.PathValue("hikeId")
	leaderCode := r.URL.Query().Get("leaderCode")

	_, err := db.Exec("UPDATE hikes SET status = 'closed' WHERE leader_code = ?", leaderCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Force all participants to finished (do we want this?)
	_, err = db.Exec(`UPDATE hike_users SET status = 'finished'
					  WHERE hike_join_code = ? AND (status = 'active' OR status = 'rsvp')
					 `, joinCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	logAction(fmt.Sprintf("Hike closed"))
}

func rsvpToHikeHandler(w http.ResponseWriter, r *http.Request) { // Renamed function
	joinCode := r.PathValue("hikeId")

	// Extract json User
	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	// fmt.Printf("%+v\n", user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get hike
	var hike Hike
	var trailheadMapLink sql.NullString
	err = db.QueryRow(`SELECT h.status, h.name, h.organization, h.trailhead_name, u.name, u.phone, h.trailhead_map_link, h.start_time, h.join_code
					   FROM hikes AS h JOIN users AS u ON leader_uuid = uuid
					   WHERE h.join_code = ?
					   `, joinCode).Scan(&hike.Status, &hike.Name, &hike.Organization, &hike.TrailheadName, &hike.Leader.Name, &hike.Leader.Phone, &trailheadMapLink, &hike.StartTime, &hike.JoinCode)

	// Check if the hike exists and is open
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Hike not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if hike.Status != "open" {
		http.Error(w, "Hike has already ended", http.StatusBadRequest)
		return
	}

	// Insert or update user in the database
	_, err = db.Exec(`
		INSERT OR REPLACE INTO users (uuid, name, phone, license_plate, emergency_contact)
		VALUES (?, ?, ?, ?, ?)
		`, user.UUID, user.Name, user.Phone, user.LicensePlate, user.EmergencyContact)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Add the participant to the hike with status rsvp
	result, err := db.Exec(`
		INSERT OR REPLACE INTO hike_users (hike_join_code, user_uuid, status, joined_at)
		VALUES (?, ?, 'rsvp', CURRENT_TIMESTAMP)
	`, joinCode, user.UUID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hike.ParticipantId, err = result.LastInsertId()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate waiver text using the new helper function
	waiverText, err := generateWaiverText(joinCode)
	if err != nil {
		// Log the error but proceed with joining the hike, as waiver signing is secondary
		// However, if the waiver can't be generated, it's a significant issue.
		log.Printf("Critical: Error generating waiver text for hike %s: %v", joinCode, err)
		// Depending on policy, we might want to return an error to the user here.
		// For now, we'll log it and proceed with an empty waiverText to not break the flow,
		// but this means the stored waiver will be incorrect/empty.
		// A better approach might be to return HTTP 500 if waiver generation fails.
		// For now, let's make waiverText empty and log, but this is a point of consideration.
		waiverText = "" // Or handle error more gracefully, e.g., http.Error
	}

	// Get User-Agent
	userAgent := r.UserAgent()
	if userAgent == "" { // Fallback, though UserAgent() usually provides a value
		userAgent = r.Header.Get("User-Agent")
	}

	// Get IP Address
	ipAddress := r.Header.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	} else {
		// X-Forwarded-For can be a comma-separated list of IPs.
		// The first IP is generally the client's IP.
		ips := strings.Split(ipAddress, ",")
		ipAddress = strings.TrimSpace(ips[0])
	}

	// Insert waiver signature
	_, err = db.Exec(`
		INSERT OR REPLACE INTO waiver_signatures
               (user_uuid, hike_join_code, signed_at, user_agent, ip_address, waiver_text)
		VALUES (?, ?, ?, ?, ?, ?)
	`, user.UUID, joinCode, time.Now().Format("2006-01-02T15:04:05-07:00"), userAgent, ipAddress, waiverText)

	if err != nil {
		// Log the error but don't fail the entire join operation,
		// as the user is already in hike_users.
		// Potentially, you might want to roll back the hike_users insertion
		// if waiver signing is absolutely critical, but that adds complexity.
		log.Printf("Error inserting waiver signature: %v. User: %s, Hike: %s", err, user.UUID, joinCode)
	}

	logAction(fmt.Sprintf("Participant RSVPd to hike: %s (Hike Join Code: %s), Waiver Signed", user.Name, hike.JoinCode)) // Updated log message
	json.NewEncoder(w).Encode(hike)
}

// unRSVPHandler allows a user to remove their RSVP if their status is 'rsvp'
func unRSVPHandler(w http.ResponseWriter, r *http.Request) {
	joinCode := r.PathValue("hikeId")
	participantIdStr := r.PathValue("participantId")

	// Convert participantIdStr to int64
	participantId, err := parseInt64(participantIdStr)
	if err != nil {
		http.Error(w, "Invalid participant ID format", http.StatusBadRequest)
		return
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Failed to start transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback() // Rollback if not committed

	// Fetch user_uuid and current status using participantId and joinCode
	var userUUID string
	var currentStatus string
	err = tx.QueryRow(`SELECT user_uuid, status FROM hike_users WHERE id = ? AND hike_join_code = ?`, participantId, joinCode).Scan(&userUUID, &currentStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Participant not found for this hike with the given ID.", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching participant details: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if currentStatus != "rsvp" {
		http.Error(w, fmt.Sprintf("Cannot unRSVP. Participant status is '%s', not 'rsvp'. Only users who RSVPd can unRSVP.", currentStatus), http.StatusBadRequest)
		return
	}

	// Delete from hike_users using participantId
	result, err := tx.Exec(`DELETE FROM hike_users
							 WHERE id = ? AND hike_join_code = ? AND status = 'rsvp'`,
		participantId, joinCode)
	if err != nil {
		http.Error(w, "Failed to delete participant from hike: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Failed to check rows affected for hike_users deletion: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		// This might happen if status changed between SELECT and DELETE, or ID/joinCode mismatch
		http.Error(w, "Could not remove RSVP. Participant not found, status was not 'rsvp', or ID/hike mismatch.", http.StatusNotFound)
		return
	}

	// Delete from waiver_signatures using the fetched userUUID
	_, err = tx.Exec(`DELETE FROM waiver_signatures
					   WHERE hike_join_code = ? AND user_uuid = ?`,
		joinCode, userUUID)
	if err != nil {
		// Log this error, but the primary action (removing from hike_users) succeeded.
		// Depending on policy, this might be considered a critical failure requiring rollback,
		// but waiver cleanup is secondary to unRSVPing from the hike itself.
		log.Printf("Error deleting waiver signature for user %s, hike %s (participantId %d): %v. Continuing with unRSVP.", userUUID, joinCode, participantId, err)
		// If this should be a hard failure, uncomment the following:
		// http.Error(w, "Failed to delete waiver signature: "+err.Error(), http.StatusInternalServerError)
		// return
	}

	err = tx.Commit()
	if err != nil {
		http.Error(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	logAction(fmt.Sprintf("Participant with ID %d (UserUUID: %s) unRSVPd from hike %s", participantId, userUUID, joinCode))
}

// Helper function to parse string to int64 (could be in a utils package)
func parseInt64(s string) (int64, error) {
	var i int64
	_, err := fmt.Sscan(s, &i)
	return i, err
}

// Given a leader code, return all participants of the hike
func getHikeParticipantsHandler(w http.ResponseWriter, r *http.Request) {
	leaderCode := r.URL.Query().Get("leaderCode")

	var participants []Participant

	rows, err := db.Query(`
		SELECT
		  u.name,
		  u.phone,
		  u.license_plate,
		  u.emergency_contact,
		  hu.status,
          hu.id,
		  ws.signed_at
		FROM
		  hike_users hu
		  JOIN users u ON hu.user_uuid = u.uuid
          JOIN waiver_signatures ws
            ON
              hu.user_uuid = ws.user_uuid AND
              hu.hike_join_code = (SELECT join_code FROM hikes WHERE leader_code =?)
		WHERE
		  hu.hike_join_code = (SELECT join_code FROM hikes WHERE leader_code = ?)`,
		leaderCode, leaderCode)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var p Participant
		var dateTimeString string
		err := rows.Scan(&p.User.Name, &p.User.Phone, &p.User.LicensePlate, &p.User.EmergencyContact, &p.Status, &p.Id, &dateTimeString)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		p.Waiver, err = time.Parse("2006-01-02T15:04:05-07:00", dateTimeString)
		if err != nil {
			logAction(fmt.Sprintf("Error parsing date: %s", err.Error()))
		}
		participants = append(participants, p)
	}

	json.NewEncoder(w).Encode(participants)
}

// TODO: For security require either LeaderCode or User's UUID
func updateParticipantStatusHandler(w http.ResponseWriter, r *http.Request) {
	joinCode := r.PathValue("hikeId")
	participantId := r.PathValue("participantId")

	var request struct {
		Status string `json:"status"`
	}

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := db.Exec(`UPDATE hike_users
					  SET status = ?
					  WHERE
                        hike_join_code = (SELECT join_code FROM hikes
                                          WHERE join_code = ? and status = "open") AND
                        id = ?
					 `, request.Status, joinCode, participantId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		http.Error(w, "Hike not found or not open", http.StatusBadRequest)
	}

	//w.WriteHeader(http.StatusOK)
	logAction(fmt.Sprintf("Participant status updated: Id: %s, Leader Code: %s, New Status: %s", participantId, joinCode, request.Status))
}

// getHikesHandler returns hikes based on query parameters:
// - latitude, longitude: nearby hikes
// - userUUID: hikes user has RSVPd to
// - leaderID: hikes led by the leader
func getHikesHandler(w http.ResponseWriter, r *http.Request) {
	// latitude := r.URL.Query().Get("latitude") // Removed
	// longitude := r.URL.Query().Get("longitude") // Removed
	userUUID := r.URL.Query().Get("userUUID")
	// leaderID parameter is removed

	var allHikes []Hike
	// now := time.Now() // For time-based filtering - Removed as only user-specific hikes don't need this complex time window here

	// Fetch by location logic removed

	// Fetch by userUUID (RSVP'd hikes)
	if userUUID != "" {
		rows, err := db.Query(`
			SELECT h.name, h.organization, h.trailhead_name, h.trailhead_map_link, h.start_time, h.join_code, h.status, h.description,
			       hu.id AS participant_id, l.uuid AS leader_uuid, l.name AS leader_name, l.phone AS leader_phone
			FROM hikes AS h
			JOIN hike_users AS hu ON h.join_code = hu.hike_join_code
			JOIN users AS l ON h.leader_uuid = l.uuid
			WHERE hu.user_uuid = ? AND hu.status = 'rsvp' AND h.status = 'open'
			ORDER BY h.start_time DESC
		`, userUUID)

		if err != nil {
			http.Error(w, "Error querying RSVP hikes: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var h Hike
			var descriptionMarkdown sql.NullString
			var trailheadMapLink sql.NullString
			err := rows.Scan(
				&h.Name, &h.Organization, &h.TrailheadName, &trailheadMapLink, &h.StartTime, &h.JoinCode, &h.Status, &descriptionMarkdown,
				&h.ParticipantId, &h.Leader.UUID, &h.Leader.Name, &h.Leader.Phone,
			)
			if err != nil {
				http.Error(w, "Error scanning RSVP hike: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if descriptionMarkdown.Valid {
				h.DescriptionMarkdown = descriptionMarkdown.String
				var buf strings.Builder
				if err := goldmark.Convert([]byte(h.DescriptionMarkdown), &buf); err == nil {
					p := bluemonday.UGCPolicy()
					h.DescriptionHTML = p.Sanitize(buf.String())
				} else {
					log.Printf("Error converting description to HTML for rsvp hike %s: %v", h.JoinCode, err)
					h.DescriptionHTML = h.DescriptionMarkdown // Fallback
				}
			} else {
				h.DescriptionMarkdown = ""
				h.DescriptionHTML = ""
			}
			if trailheadMapLink.Valid {
				h.TrailheadMapLink = trailheadMapLink.String
			}
			h.SourceType = "rsvp"
			allHikes = append(allHikes, h)
		}
		if err = rows.Err(); err != nil {
			http.Error(w, "Error iterating RSVP hikes: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// If userUUID is provided, also fetch hikes led by this user
	if userUUID != "" {
		rows, err := db.Query(`
			SELECT h.join_code, h.name, h.organization, h.trailhead_name, u.uuid as leader_uuid, u.name AS leader_name, u.phone AS leader_phone,
			       h.trailhead_map_link, h.start_time, h.status, h.leader_code, h.description
			FROM hikes AS h
			JOIN users AS u ON h.leader_uuid = u.uuid
			WHERE h.leader_uuid = ? AND h.status = 'open'
			ORDER BY h.start_time DESC
		`, userUUID) // Query by userUUID for hikes they are leading

		if err != nil {
			http.Error(w, "Error querying hikes led by user: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var h Hike
			var descriptionMarkdown sql.NullString
			var trailheadMapLink sql.NullString
			err := rows.Scan(
				&h.JoinCode, &h.Name, &h.Organization, &h.TrailheadName, &h.Leader.UUID, &h.Leader.Name, &h.Leader.Phone,
				&trailheadMapLink, &h.StartTime, &h.Status, &h.LeaderCode, &descriptionMarkdown, // Added h.LeaderCode
			)
			if err != nil {
				http.Error(w, "Error scanning hike led by user: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if descriptionMarkdown.Valid {
				h.DescriptionMarkdown = descriptionMarkdown.String
				var buf strings.Builder
				if err := goldmark.Convert([]byte(h.DescriptionMarkdown), &buf); err == nil {
					p := bluemonday.UGCPolicy()
					h.DescriptionHTML = p.Sanitize(buf.String())
				} else {
					log.Printf("Error converting description to HTML for led_by_user hike %s: %v", h.JoinCode, err)
					h.DescriptionHTML = h.DescriptionMarkdown // Fallback
				}
			} else {
				h.DescriptionMarkdown = ""
				h.DescriptionHTML = ""
			}
			if trailheadMapLink.Valid {
				h.TrailheadMapLink = trailheadMapLink.String
			}
			h.SourceType = "led_by_user" // New SourceType
			allHikes = append(allHikes, h)
		}
		if err = rows.Err(); err != nil {
			http.Error(w, "Error iterating hikes led by user: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Note: As per plan, if a hike matches multiple criteria, it will appear multiple times
	// in allHikes, each with its respective SourceType. No deduplication is done.

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allHikes)
}

// Given a query string, return a list of trailhead suggestions
func trailheadSuggestionsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		json.NewEncoder(w).Encode([]Trailhead{})
		return
	}

	rows, err := db.Query("SELECT name, map_link FROM trailheads WHERE REPLACE(name, '''', '') LIKE ? LIMIT 5", "%"+query+"%")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var suggestions []Trailhead
	for rows.Next() {
		var th Trailhead
		if err := rows.Scan(&th.Name, &th.MapLink); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		suggestions = append(suggestions, th)
	}

	json.NewEncoder(w).Encode(suggestions)
}

func generateSecureLinkCode() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func logAction(action string) {
	f, err := os.OpenFile("hiketracker.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()

	logger := log.New(f, "", log.LstdFlags)
	logger.Println(action)
}
