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
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Keep in sync with trailheads table schema
type Trailhead struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// List of predefined trailheads
var predefinedTrailheads = []Trailhead{
	{Name: "Ka'ena Point Trailhead", Latitude: 21.57899, Longitude: -158.23760},
	{Name: "Kolowalu Trailhead", Latitude: 21.32160, Longitude: -157.79864},
	{Name: "Konahuanui Trailhead", Latitude: 21.33057, Longitude: -157.82144},
	{Name: "Upper Lulumahu Falls Trail", Latitude: 21.33057, Longitude: -157.82144},
	{Name: "Tantalus Ramble Trail", Latitude: 21.33057, Longitude: -157.82144},
	{Name: "Manana Ridge Trail", Latitude: 21.43038, Longitude: -157.93892},
	{Name: "Iliahi Ridge Trail", Latitude: 21.43038, Longitude: -157.93892},
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
	TrailheadName string    `json:"trailheadName"`
	Leader        User      `json:"leader"`
	Latitude      float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	CreatedAt  time.Time `json:"-"` // don't send this field in JSON response
	StartTime  time.Time `json:"startTime"`
	Status     string    `json:"Status"`
	JoinCode   string    `json:"joinCode"`
	LeaderCode string    `json:"leaderCode"`
}

// Keep in sync with participants table schema
type Participant struct {
	Hike     Hike      `json:"hike"`
	User     User      `json:"user"`
	Status   string    `json:"status"`
	JoinedAt time.Time `json:"joinedAt"`
}

var db *sql.DB

// Database initialization should be in its own function so that it can be called from tests rather than in init()
func initDB(databaseName string) {
	var err error
	db, err = sql.Open("sqlite3", databaseName)
	if err != nil {
		log.Fatal(err)
	}

	// Create tables if they don't exist
	// Note to self: Foreign key declarations must be at the end of the table creation statement
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS trailheads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			latitude REAL,
			longitude REAL
		);

		CREATE TABLE IF NOT EXISTS users (
			uuid TEXT PRIMARY KEY,
			name TEXT,
			phone TEXT,
			license_plate TEXT,
			emergency_contact TEXT
		);

		CREATE TABLE IF NOT EXISTS hikes (
			name TEXT,
			trailhead_name TEXT,
			leader_uuid TEXT,
			latitude REAL,
			longitude REAL,
			created_at DATETIME,
			start_time DATETIME,
			status TEXT DEFAULT 'open',
			join_code TEXT PRIMARY KEY,
			leader_code TEXT UNIQUE,
			FOREIGN KEY (leader_uuid) REFERENCES users(uuid)
		);

		CREATE TABLE IF NOT EXISTS hike_users (
			hike_join_code TEXT,
			user_uuid TEXT,
			status TEXT DEFAULT 'active',
			joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (hike_join_code, user_uuid),
			FOREIGN KEY (hike_join_code) REFERENCES hikes(join_code),
			FOREIGN KEY (user_uuid) REFERENCES users(uuid)
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	// Populate trailheads table if empty
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM trailheads").Scan(&count)
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

// Add routes to ServeMux (sparate function so it can be used in testing)
func addRoutes(mux *http.ServeMux) {
	// You must define most specific routes first
	mux.HandleFunc("PUT /api/hike/{hikeId}/participant/{participantId}", updateParticipantStatusHandler)
	mux.HandleFunc("POST /api/hike/{hikeId}/participant", joinHikeHandler)
	mux.HandleFunc("GET /api/hike/{hikeId}/participant", getHikeParticipantsHandler)
	mux.HandleFunc("GET /api/hike/{hikeId}", getHikeHandler)
	mux.HandleFunc("PUT /api/hike/{hikeId}", endHikeHandler) // require leader code
	mux.HandleFunc("POST /api/hike", createHikeHandler)
	mux.HandleFunc("GET /api/hike", getNearbyHikesHandler)
	mux.HandleFunc("GET /api/trailhead", trailheadSuggestionsHandler)
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
	var hike Hike
	err := json.NewDecoder(r.Body).Decode(&hike)
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

	// Insert or update user in the database
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
	_, err = db.Exec(`
		INSERT INTO hikes (name, leader_uuid, latitude, longitude, created_at, start_time, join_code, leader_code)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, hike.Name, hike.Leader.UUID, hike.Latitude, hike.Longitude, hike.CreatedAt, hike.StartTime, hike.JoinCode, hike.LeaderCode)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(hike)
	logAction(fmt.Sprintf("Hike created: %s by %s, starting at %s", hike.Name, hike.Leader.Name, hike.StartTime.Format(time.RFC3339)))
}

// Get hike details by join code, Don't return leader code
func getHikeHandler(w http.ResponseWriter, r *http.Request) {
	joinCode := r.PathValue("hikeId")
	leaderCode := r.URL.Query().Get("leaderCode")

	var hike Hike
	var err error
	if leaderCode != "" {
		err = db.QueryRow(`SELECT h.name, u.name, u.phone, h.latitude, h.longitude, h.start_time, h.join_code
		                   FROM hikes As h JOIN users AS u ON leader_uuid = uuid
		                   WHERE h.leader_code = ? AND h.status = "open"
		`, leaderCode).Scan(&hike.Name, &hike.Leader.Name, &hike.Leader.Phone, &hike.Latitude, &hike.Longitude, &hike.StartTime, &hike.JoinCode)
	} else {
		err = db.QueryRow(`SELECT h.name, u.name, u.phone, h.latitude, h.longitude, h.start_time, h.join_code
						   FROM hikes As h JOIN users AS u ON leader_uuid = uuid
						   WHERE h.join_code = ? AND h.status = "open"
		`, joinCode).Scan(&hike.Name, &hike.Leader.Name, &hike.Leader.Phone, &hike.Latitude, &hike.Longitude, &hike.StartTime, &hike.JoinCode)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Hike not found or already closed", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	json.NewEncoder(w).Encode(hike)
}

// End a hike and mark all participants as finished
func endHikeHandler(w http.ResponseWriter, r *http.Request) {
	var hike Hike

	err := json.NewDecoder(r.Body).Decode(&hike)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = db.Exec("UPDATE hikes SET status = 'closed' WHERE leader_code = ?", hike.LeaderCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = db.Exec(`UPDATE hike_users SET status = 'finished'
					  WHERE hike_join_code = ? AND status = 'active'
					 `, hike.JoinCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	logAction(fmt.Sprintf("Hike closed: Name %s", hike.Name))
}

func joinHikeHandler(w http.ResponseWriter, r *http.Request) {
	joinCode := r.PathValue("hikeId")

	var request struct {
		User User `json:"user"`
	}

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the hike exists and is open
	var hike Hike
	err = db.QueryRow(`SELECT h.status, h.name, u.name, u.phone, h.latitude, h.longitude, h.start_time, h.join_code
					   FROM hikes AS h JOIN users AS u ON leader_uuid = uuid
					   WHERE h.join_code = ?
					   `, joinCode).Scan(&hike.Status, &hike.Name, &hike.Leader.Name, &hike.Leader.Phone, &hike.Latitude, &hike.Longitude, &hike.StartTime, &hike.JoinCode)

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
		`, request.User.UUID, request.User.Name, request.User.Phone, request.User.LicensePlate, request.User.EmergencyContact)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Add the participant to the hike
	_, err = db.Exec(`
		INSERT OR REPLACE INTO hike_users (hike_join_code, user_uuid, joined_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`, joinCode, request.User.UUID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logAction(fmt.Sprintf("Participant joined hike: %s (Hike Join Code: %s)", request.User.Name, hike.JoinCode))
	json.NewEncoder(w).Encode(hike)
}

// Given a leader code, return all participants of the hike
func getHikeParticipantsHandler(w http.ResponseWriter, r *http.Request) {
	leaderCode := r.URL.Query().Get("leaderCode")

	var participants []Participant

	rows, err := db.Query(`SELECT uuid, name, phone, license_plate, emergency_contact, status
						   FROM hike_users JOIN users ON user_uuid = uuid
						   WHERE hike_join_code = (SELECT join_code FROM hikes WHERE leader_code = ?)
						  `, leaderCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var p Participant
		err := rows.Scan(&p.User.UUID, &p.User.Name, &p.User.Phone, &p.User.LicensePlate, &p.User.EmergencyContact, &p.Status)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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
		SELECT h.join_code, h.name, u.name, u.phone, h.latitude, h.longitude, h.start_time
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
		err := rows.Scan(&h.JoinCode, &h.Name, &h.Leader.Name, &h.Leader.Phone, &h.Latitude, &h.Longitude, &h.StartTime)
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

	rows, err := db.Query("SELECT id, name, latitude, longitude FROM trailheads WHERE name LIKE ? LIMIT 5", "%"+query+"%")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var suggestions []Trailhead
	for rows.Next() {
		var th Trailhead
		if err := rows.Scan(&th.ID, &th.Name, &th.Latitude, &th.Longitude); err != nil {
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
