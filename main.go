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
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Keep in sync with trailheads table schema
type Trailhead struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// List of predefined trailheads
var predefinedTrailheads = []Trailhead{
	{Name: "Aiea Loop (upper)", Latitude: 21.39880, Longitude: -157.90022},
	{Name: "Aihualama (Lyon Arboretum)", Latitude: 21.3323, Longitude: -157.8016},
	{Name: "Bowman (Radar Hill)", Latitude: 21.34992, Longitude: -157.87685},
	{Name: "Crouching Lion (Manamana)", Latitude: 21.55816, Longitude: -157.86619},
	{Name: "Diamond Head Crater (Le'ahi)", Latitude: 21.26360, Longitude: -157.80603},
	{Name: "Ehukai Pillbox (Sunset Pillbox)", Latitude: 21.66465, Longitude: -158.04936},
	{Name: "Friendship Garden", Latitude: 21.40622, Longitude: -157.77751},
	{Name: "Haha'ione", Latitude: 21.310139, Longitude: -157.712835},
	{Name: "Hamana Falls", Latitude: 21.45293, Longitude: -157.85281},
	{Name: "Hau'ula Loop", Latitude: 21.60980, Longitude: -157.91544},
	{Name: "Hawaii Loa Ridge", Latitude: 21.29749, Longitude: -157.74593},
	{Name: "Ho'omaluhia Botanical Garden", Latitude: 21.38647, Longitude: -157.80956},
	{Name: "Judd", Latitude: 21.34717, Longitude: -157.82082},
	{Name: "Ka'au Crater", Latitude: 21.31108, Longitude: -157.78189},
	{Name: "Ka'ena Point (Mokule'ia Side)", Latitude: 21.57976, Longitude: -158.23773},
	{Name: "Ka'ena Point (Waianae Side)", Latitude: 21.55673, Longitude: -158.24884},
	{Name: "Kahana Valley", Latitude: 21.55023, Longitude: -157.88163},
	{Name: "Kahekili Ridge", Latitude: 21.55410, Longitude: -157.85579},
	{Name: "Kaipapa'u Gulch", Latitude: 21.61809, Longitude: -157.91893},
	{Name: "Ka'iwa Ridge (Lanikai Side)", Latitude: 21.39031, Longitude: -157.71943},
	{Name: "Ka'iwa Ridge (Keolu Side)", Latitude: 21.38174, Longitude: -157.72553},
	{Name: "Kalawahine", Latitude: 21.33125, Longitude: -157.82128},
	{Name: "Kamana'iki", Latitude: 21.34960, Longitude: -157.85821},
	{Name: "Kamilo'iki", Latitude: 21.300515, Longitude: -157.692755},
	{Name: "Kaniakapupu Ruins", Latitude: 21.351083, Longitude: -157.81698},
	{Name: "Kapa'ele'ele", Latitude: 21.55501, Longitude: -157.87682},
	{Name: "Kapena Falls", Latitude: 21.32401, Longitude: -157.84699},
	{Name: "Kaunala", Latitude: 21.64290, Longitude: -158.02590},
	{Name: "Kealia", Latitude: 21.57750, Longitude: -158.20816},
	{Name: "Kea'au Middle Ridge", Latitude: 21.50296, Longitude: -158.22544},
	{Name: "Koko Crater (Arch)", Latitude: 21.28069, Longitude: -157.67854},
	{Name: "Koko Crater (Railway)", Latitude: 21.28117, Longitude: -157.69192},
	{Name: "Koko Head (Hanauma)", Latitude: 21.27532, Longitude: -157.69363},
	{Name: "Koloa Gulch", Latitude: 21.62817, Longitude: -157.923531},
	{Name: "Kuliʻouʻou Ridge", Latitude: 21.30343, Longitude: -157.72426},
	{Name: "Kulepeamoa Ridge", Latitude: 21.29218, Longitude: -157.74093},
	{Name: "Laie Falls (parking)", Latitude: 21.65053, Longitude: -157.93147},
	{Name: "Lanihuli", Latitude: 21.33986, Longitude: -157.84751},
	{Name: "Lanipo", Latitude: 21.29787, Longitude: -157.78574},
	{Name: "Likeke Falls (First Pres)", Latitude: 21.37281, Longitude: -157.79209},
	{Name: "Lulumahu Falls", Latitude: 21.354438, Longitude: -157.81114},
	{Name: "Makapu'u Point Lighthouse", Latitude: 21.30499, Longitude: -157.65480},
	{Name: "Makiki Valley Loop (Nature Center)", Latitude: 21.31717, Longitude: -157.82700},
	{Name: "Manana Ridge", Latitude: 21.43038, Longitude: -157.93889},
	{Name: "Manoa Cliff", Latitude: 21.32612, Longitude: -157.81308},
	{Name: "Manoa Falls", Latitude: 21.33255, Longitude: -157.80055},
	{Name: "Maunawili Falls", Latitude: 21.35929, Longitude: -157.76355},
	{Name: "Maunawili Demonstration (Pali)", Latitude: 21.36496, Longitude: -157.77998},
	{Name: "Maunawili Ditch (Wakupanaha)", Latitude: 21.34294, Longitude: -157.74341},
	{Name: "Maunawili Ditch (Mahiku)", Latitude: 21.34918, Longitude: -157.73400},
	{Name: "Moanalua Valley", Latitude: 21.37412, Longitude: -157.88061},
	{Name: "Mount Ka'ala", Latitude: 21.47597, Longitude: -158.15193},
	{Name: "Nahuina", Latitude: 21.32978, Longitude: -158.82265},
	{Name: "Ohana Bike (N)", Latitude: 21.37203, Longitude: -157.74520},
	{Name: "Ohana Bike (S)", Latitude: 21.35772, Longitude: -157.73318},
	{Name: "Olomana", Latitude: 21.36845, Longitude: -157.76097},
	{Name: "Pali Notches", Latitude: 21.36670, Longitude: -157.79322},
	{Name: "Pali Puka", Latitude: 21.36682, Longitude: -157.79417},
	{Name: "Puʻu Māʻeliʻel", Latitude: 21.43429, Longitude: -157.82463},
	{Name: "Pu'u Manamana", Latitude: 21.55410, Longitude: -157.85579},
	{Name: "Pu'u Ohia", Latitude: 21.33109, Longitude: -157.81465},
	{Name: "Pu'u O Hulu (Pink Pillbox)", Latitude: 21.40478, Longitude: -158.17268},
	{Name: "Pu'u Pia Trail", Latitude: 21.32168, Longitude: -157.79873},
	{Name: "Tantalus Arboretum", Latitude: 21.32582, Longitude: -157.82771},
	{Name: "Tom Tom", Latitude: 21.32499, Longitude: -157.69683},
	{Name: "Ualakaa", Latitude: 21.31645, Longitude: -157.82037},
	{Name: "Wa'ahila Ridge", Latitude: 21.30729, Longitude: -157.79765},
	{Name: "Wahiawa Hills", Latitude: 21.50846, Longitude: -157.98648},
	{Name: "Waiau (parking)", Latitude: 21.41257, Longitude: -157.93985},
	{Name: "Wailupe Valley (Hao)", Latitude: 21.29861, Longitude: -157.75663},
	{Name: "Wailupe Valley (Mona)", Latitude: 21.29999, Longitude: -157.75466},
	{Name: "Waimalu Ditch", Latitude: 21.39888, Longitude: -157.91763},
	{Name: "Waimano Ridge", Latitude: 21.41725, Longitude: -157.95104},
	{Name: "Waipuilani Falls", Latitude: 21.3643, Longitude: -157.7959},
	{Name: "Waipuhia Falls", Latitude: 21.36173, Longitude: -157.80544},
	{Name: "Wiliwilinui Ridge", Latitude: 21.29927, Longitude: -157.76274},
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
	Name          string    `json:"name"` // Custom name for the hike event
	Organization  string    `json:"organization"`
	TrailheadName string    `json:"trailheadName"`
	Leader        User      `json:"leader"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	CreatedAt     time.Time `json:"-"` // don't send this field in JSON response
	StartTime     time.Time `json:"startTime"`
	Status        string    `json:"Status"`
	JoinCode      string    `json:"joinCode"`
	LeaderCode    string    `json:"leaderCode"`
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
			latitude REAL NOT NULL,
			longitude REAL NOT NULL
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
			latitude REAL,
			longitude REAL,
			created_at DATETIME NOT NULL,
			start_time DATETIME NOT NULL,
			status TEXT DEFAULT 'open',
			join_code TEXT PRIMARY KEY,
			leader_code TEXT UNIQUE,
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
			_, err := db.Exec("INSERT INTO trailheads (name, latitude, longitude) VALUES (?, ?, ?)", trailhead.Name, trailhead.Latitude, trailhead.Longitude)
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
	mux.HandleFunc("POST /api/hike/{hikeId}/rsvp", rsvpToHikeHandler) // pass in User
	mux.HandleFunc("POST /api/hike/{hikeId}/participant/{userUUID}/start", startHikingHandler)
	mux.HandleFunc("DELETE /api/hike/{hikeId}/participant/{userUUID}/rsvp", unRSVPHandler)
	mux.HandleFunc("GET /api/hike/{hikeId}/participant", getHikeParticipantsHandler)
	mux.HandleFunc("GET /api/hike/{hikeId}", getHikeHandler)
	mux.HandleFunc("PUT /api/hike/{hikeId}", endHikeHandler) // require leader code
	mux.HandleFunc("POST /api/hike", createHikeHandler)
	mux.HandleFunc("GET /api/hike", getNearbyHikesHandler)
	mux.HandleFunc("GET /api/trailhead", trailheadSuggestionsHandler)
	mux.HandleFunc("GET /api/userhikes/{userUUID}", getUserHikesByStatusHandler)
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

	// TODO: If hike with same trail, leader_name and open status already exists then return error

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

	// TODO: If join or leader code already exist, generate new codes

	// Add hike to the HIkes table
	_, err = db.Exec(`
		INSERT INTO hikes (name, organization, trailhead_name, leader_uuid, latitude, longitude, created_at, start_time, join_code, leader_code)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, hike.Name, hike.Organization, hike.TrailheadName, hike.Leader.UUID, hike.Latitude, hike.Longitude, hike.CreatedAt, hike.StartTime, hike.JoinCode, hike.LeaderCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return Hike to the caller
	json.NewEncoder(w).Encode(hike)
	logAction(fmt.Sprintf("Hike created: %s by %s, starting at %s", hike.Name, hike.Leader.Name, hike.StartTime.Format(time.RFC3339)))
}

// Get hike details by join code, Don't return leader code
func getHikeHandler(w http.ResponseWriter, r *http.Request) {
	joinCode := r.PathValue("hikeId")
	leaderCode := r.URL.Query().Get("leaderCode")

	// Retrieve Hike record based on leaderCode if provided otherwise by joinCode
	var hike Hike
	var err error
	if leaderCode != "" {
		err = db.QueryRow(`SELECT h.name, h.organization, h.trailhead_name, u.name, u.phone, h.latitude, h.longitude, h.start_time, h.join_code
		                   FROM hikes As h JOIN users AS u ON leader_uuid = uuid
		                   WHERE h.leader_code = ? AND h.status = "open"
		`, leaderCode).Scan(&hike.Name, &hike.Organization, &hike.TrailheadName, &hike.Leader.Name, &hike.Leader.Phone, &hike.Latitude, &hike.Longitude, &hike.StartTime, &hike.JoinCode)
	} else {
		err = db.QueryRow(`SELECT h.name, h.organization, h.trailhead_name, u.name, u.phone, h.latitude, h.longitude, h.start_time, h.join_code
						   FROM hikes As h JOIN users AS u ON leader_uuid = uuid
						   WHERE h.join_code = ? AND h.status = "open"
		`, joinCode).Scan(&hike.Name, &hike.Organization, &hike.TrailheadName, &hike.Leader.Name, &hike.Leader.Phone, &hike.Latitude, &hike.Longitude, &hike.StartTime, &hike.JoinCode)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Hike not found or already closed", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
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
	err = db.QueryRow(`SELECT h.status, h.name, h.organization, h.trailhead_name, u.name, u.phone, h.latitude, h.longitude, h.start_time, h.join_code
					   FROM hikes AS h JOIN users AS u ON leader_uuid = uuid
					   WHERE h.join_code = ?
					   `, joinCode).Scan(&hike.Status, &hike.Name, &hike.Organization, &hike.TrailheadName, &hike.Leader.Name, &hike.Leader.Phone, &hike.Latitude, &hike.Longitude, &hike.StartTime, &hike.JoinCode)

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
	_, err = db.Exec(`
		INSERT OR REPLACE INTO hike_users (hike_join_code, user_uuid, status, joined_at)
		VALUES (?, ?, 'rsvp', CURRENT_TIMESTAMP)
	`, joinCode, user.UUID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO: Replace this waiver stuff to use template
	// Read waiver text
	waiverTextBytes, err := os.ReadFile("static/waiver.txt")
	if err != nil {
		// Log the error but proceed with joining the hike, as waiver signing is secondary
		log.Printf("Error reading waiver.txt: %v", err)
		// Potentially send a different response or log more critically depending on requirements
	}
	waiverText := string(waiverTextBytes)

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
	`, user.UUID, joinCode, time.Now(), userAgent, ipAddress, waiverText)

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

// startHikingHandler allows a user to change their status from 'rsvp' to 'active'
func startHikingHandler(w http.ResponseWriter, r *http.Request) {
	joinCode := r.PathValue("hikeId")
	userUUID := r.PathValue("userUUID")

	// Check if the hike exists and is open
	var hikeStatus string
	err := db.QueryRow(`SELECT status FROM hikes WHERE join_code = ?`, joinCode).Scan(&hikeStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Hike not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// TODO: add check that we are not more than an hour before the start hike time.
	if hikeStatus != "open" {
		http.Error(w, "Hike is not open. Cannot start hiking.", http.StatusBadRequest)
		return
	}

	// Update participant status from 'rsvp' to 'active'
	result, err := db.Exec(`UPDATE hike_users
							 SET status = 'active'
							 WHERE hike_join_code = ? AND user_uuid = ? AND status = 'rsvp'`,
		joinCode, userUUID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Failed to check rows affected", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		// This could be because the user/hike combination doesn't exist, or status wasn't 'rsvp'
		// Query current status to give a more specific error
		var currentStatus string
		err := db.QueryRow(`SELECT status FROM hike_users WHERE hike_join_code = ? AND user_uuid = ?`, joinCode, userUUID).Scan(&currentStatus)
		if err == sql.ErrNoRows {
			http.Error(w, "Participant not found for this hike.", http.StatusNotFound)
		} else if err != nil {
			http.Error(w, "Error fetching participant status: "+err.Error(), http.StatusInternalServerError)
		} else if currentStatus != "rsvp" {
			http.Error(w, fmt.Sprintf("Cannot start hiking. Participant status is '%s', not 'rsvp'.", currentStatus), http.StatusBadRequest)
		} else {
			// Should not happen if rowsAffected was 0 and status was 'rsvp'
			http.Error(w, "Could not update participant status. Please ensure you have RSVPd.", http.StatusBadRequest)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	logAction(fmt.Sprintf("Participant %s started hiking for hike %s", userUUID, joinCode))
}

// unRSVPHandler allows a user to remove their RSVP if their status is 'rsvp'
func unRSVPHandler(w http.ResponseWriter, r *http.Request) {
	joinCode := r.PathValue("hikeId")
	userUUID := r.PathValue("userUUID")

	// Check if the hike exists and is open - users should be able to unRSVP even if hike is closed for new RSVPs,
	// but perhaps not if it has already ended or started. For now, let's allow unRSVP as long as hike exists.
	// However, the main check is participant status.

	var currentStatus string
	err := db.QueryRow(`SELECT status FROM hike_users WHERE hike_join_code = ? AND user_uuid = ?`, joinCode, userUUID).Scan(&currentStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Participant not found for this hike.", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching participant status: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if currentStatus != "rsvp" {
		http.Error(w, fmt.Sprintf("Cannot unRSVP. Participant status is '%s', not 'rsvp'. Only users who RSVPd can unRSVP.", currentStatus), http.StatusBadRequest)
		return
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Failed to start transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Delete from hike_users
	result, err := tx.Exec(`DELETE FROM hike_users
							 WHERE hike_join_code = ? AND user_uuid = ? AND status = 'rsvp'`,
		joinCode, userUUID)
	if err != nil {
		tx.Rollback()
		http.Error(w, "Failed to delete participant from hike: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		http.Error(w, "Failed to check rows affected for hike_users deletion: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		tx.Rollback()
		// This should ideally not happen if we've already checked the status above,
		// but it's a good safeguard (e.g. race condition, though unlikely here)
		http.Error(w, "Could not remove RSVP. Participant not found or status was not 'rsvp'.", http.StatusNotFound)
		return
	}

	// Delete from waiver_signatures
	_, err = tx.Exec(`DELETE FROM waiver_signatures
					   WHERE hike_join_code = ? AND user_uuid = ?`,
		joinCode, userUUID)
	if err != nil {
		tx.Rollback()
		http.Error(w, "Failed to delete waiver signature: "+err.Error(), http.StatusInternalServerError)
		// Note: Participant was already removed from hike_users. This is a partial failure.
		// Depending on policy, might want to log this inconsistency.
		return
	}

	err = tx.Commit()
	if err != nil {
		http.Error(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	logAction(fmt.Sprintf("Participant %s unRSVPd from hike %s", userUUID, joinCode))
}

// Given a leader code, return all participants of the hike
func getHikeParticipantsHandler(w http.ResponseWriter, r *http.Request) {
	leaderCode := r.URL.Query().Get("leaderCode")

	var participants []Participant

	// rows, err := db.Query(`SELECT uuid, name, phone, license_plate, emergency_contact, status
	// 					   FROM hike_users JOIN users ON user_uuid = uuid
	// 					   WHERE hike_join_code = (SELECT join_code FROM hikes WHERE leader_code = ?)
	// 					  `, leaderCode)

	rows, err := db.Query(`
		SELECT
		  u.uuid,
		  u.name,
		  u.phone,
		  u.license_plate,
		  u.emergency_contact,
		  hu.status,
		  ws.most_recent_waiver_date
		FROM
		  hike_users hu
		  JOIN users u ON hu.user_uuid = u.uuid
		  JOIN (
			SELECT
			  user_uuid,
			  MAX(signed_at) AS most_recent_waiver_date
			FROM
			  waiver_signatures
            WHERE
              hike_join_code = (SELECT join_code FROM hikes WHERE leader_code =?)
			GROUP BY
			  user_uuid
		  ) AS ws ON hu.user_uuid = ws.user_uuid
		WHERE
		  hu.hike_join_code = (
			SELECT
			  join_code
			FROM
			  hikes
			WHERE
			  leader_code = ?
		)`, leaderCode, leaderCode)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var p Participant
		var dateTimeString string
		err := rows.Scan(&p.User.UUID, &p.User.Name, &p.User.Phone, &p.User.LicensePlate, &p.User.EmergencyContact, &p.Status, &dateTimeString)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		p.Waiver, err = time.Parse("2006-01-02 15:04:05.000000000-07:00", dateTimeString)
		if err != nil {
			logAction(fmt.Sprintf("Error parsing date: %s", err.Error()))
		}
		participants = append(participants, p)
	}

	json.NewEncoder(w).Encode(participants)
}

func updateParticipantStatusHandler(w http.ResponseWriter, r *http.Request) {
	joinCode := r.PathValue("hikeId")
	userUUID := r.PathValue("participantId")

	var request struct {
		Status string `json:"status"`
	}

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = db.Exec(`UPDATE hike_users
					  SET status = ?
					  WHERE hike_join_code = (SELECT join_code FROM hikes WHERE join_code = ?) AND user_uuid = ?
					 `, request.Status, joinCode, userUUID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	logAction(fmt.Sprintf("Participant status updated: UUID %s, Leader Code: %s, New Status: %s", userUUID, joinCode, request.Status))
}

// Given a latitude and longitude, return all hikes within a 0.25 mile radius
func getNearbyHikesHandler(w http.ResponseWriter, r *http.Request) {
	latitude := r.URL.Query().Get("latitude")
	longitude := r.URL.Query().Get("longitude")

	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	oneHourFromNow := now.Add(1 * time.Hour)

	//	latRange = .25 / 69
	//	lonRange = .25 / (69 * math.Cos(latitude*(math.Pi/180)))

	rows, err := db.Query(`
		SELECT h.join_code, h.name, h.organization, u.name, u.phone, h.latitude, h.longitude, h.start_time
		FROM hikes AS h JOIN users AS u ON leader_uuid = uuid
		WHERE h.latitude - ? <= 0.003623
		AND h.longitude - ? <= 0.003896
		AND h.status = 'open'
		AND h.start_time BETWEEN ? AND ?
	`, latitude, longitude, oneHourAgo, oneHourFromNow)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var hikes []Hike
	for rows.Next() {
		var h Hike
		err := rows.Scan(&h.JoinCode, &h.Name, &h.Organization, &h.Leader.Name, &h.Leader.Phone, &h.Latitude, &h.Longitude, &h.StartTime)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		hikes = append(hikes, h)
	}

	json.NewEncoder(w).Encode(hikes)
}

// Given a query string, return a list of trailhead suggestions
func trailheadSuggestionsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		json.NewEncoder(w).Encode([]Trailhead{})
		return
	}

	rows, err := db.Query("SELECT name, latitude, longitude FROM trailheads WHERE REPLACE(name, '''', '') LIKE ? LIMIT 5", "%"+query+"%")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var suggestions []Trailhead
	for rows.Next() {
		var th Trailhead
		if err := rows.Scan(&th.Name, &th.Latitude, &th.Longitude); err != nil {
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

// getUserHikesByStatusHandler returns a list of hikes for a given user, filtered by status.
func getUserHikesByStatusHandler(w http.ResponseWriter, r *http.Request) {
	userUUID := r.PathValue("userUUID")
	statusFilter := r.URL.Query().Get("status")

	if statusFilter == "" {
		http.Error(w, "Missing required 'status' query parameter", http.StatusBadRequest)
		return
	}

	query := `
		SELECT h.name, h.organization, h.trailhead_name, h.latitude, h.longitude, h.start_time, h.join_code,
		       l.name AS leader_name, l.phone AS leader_phone
		FROM hikes AS h
		JOIN hike_users AS hu ON h.join_code = hu.hike_join_code
		JOIN users AS l ON h.leader_uuid = l.uuid
		WHERE hu.user_uuid = ? AND hu.status = ? AND h.status = 'open'
		ORDER BY h.start_time DESC
	`

	rows, err := db.Query(query, userUUID, statusFilter)
	if err != nil {
		http.Error(w, "Database query error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var hikes []Hike
	for rows.Next() {
		var h Hike
		// We need to scan into h.Leader.Name and h.Leader.Phone separately
		// as h.Leader is a struct.
		err := rows.Scan(
			&h.Name, &h.Organization, &h.TrailheadName, &h.Latitude, &h.Longitude, &h.StartTime, &h.JoinCode,
			&h.Leader.Name, &h.Leader.Phone,
		)
		if err != nil {
			http.Error(w, "Error scanning row: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// The userUUID for the leader is h.leader_uuid in the hikes table,
		// but we don't explicitly fetch it into h.Leader.UUID here unless needed by frontend.
		// The current Hike struct's Leader User field doesn't require UUID for this specific display.
		hikes = append(hikes, h)
	}

	if err = rows.Err(); err != nil {
		http.Error(w, "Error iterating rows: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hikes)
}
