package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"sync"
	"text/template"
	"time"

	"github.com/gofrs/uuid/v5"
	"golang.org/x/crypto/bcrypt"
)

type Sesh struct {
	Expiry time.Time
	Name   string
	Sub    string
}

var (
	// Map for storing session information
	sessions = make(map[string]Sesh)
	// Mutex to synchronize access to session information - to be locked in case of concurrent writes
	sessionMutex = &sync.Mutex{}
)

// Function to check if the user has an active session, used to check permissions
func CheckSession(w http.ResponseWriter, r *http.Request, allow bool) bool {
	// Checking for a cookie and if there is a corresponding session stored on the server side
	cookie, err := r.Cookie("session_id")
	if err == nil {
		if _, ok := sessions[cookie.Value]; ok {
			return true
		} else {
			if !allow {
				ErrorHandler(w, http.StatusForbidden, "403 Forbidden - You must log in again to perform this action.", "login.html", "Login")
			}
		}
	} else {
		if !allow {
			fmt.Println(err)
			ErrorHandler(w, http.StatusUnauthorized, "401 Unauthorized - You must be logged in to perform this action.", "login.html", "Login")
		}
	}

	return false
}

// Compare a password with a hashed password when logging in
func CheckPasswordHash(hash, pass string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass))
	if err != nil {
		fmt.Println(err)
		return false
	}

	return true
}

// Handler for users attempting to login
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Check for correct request method
	if r.Method != http.MethodPost {
		ErrorHandler(w, http.StatusMethodNotAllowed, "405 Method Not Allowed - Please submit the appropriate form.", "login.html", "Login")
		return
	}

	// Retrieve session ID from Cookie and check is user is already logged in
	cookie, err := r.Cookie("session_id")
	if err == nil {
		if _, ok := sessions[cookie.Value]; ok {
			ErrorHandler(w, http.StatusForbidden, "403 Forbidden - You must logout to perform this action.", "", "Home")
			return
		}
	}

	// User authentication checking username and password
	var hash string
	err = db.QueryRow("SELECT pass FROM users WHERE username = ?", r.FormValue("myName")).Scan(&hash)
	if err != nil && err != sql.ErrNoRows {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}
	if !CheckPasswordHash(hash, r.FormValue("myPass")) || err == sql.ErrNoRows {
		ErrorHandler(w, http.StatusUnauthorized, "401 Unauthorized - Invalid login details", "login.html", "Login")
		return
	}

	// Check if user already has a session, if so, delete the old session stored server side
	sessionMutex.Lock()
	for id, sesh := range sessions {
		if sesh.Name == r.FormValue("myName") {
			sessions[id] = Sesh{time.Now(), r.FormValue("myName"), ""}
		}
	}
	sessionMutex.Unlock()

	// Create a Version 4 UUID
	sessionID, err := uuid.NewV4()
	if err != nil {
		log.Fatalf("Failed to generate UUID: %v", err)
	}
	fmt.Println("Generated Version 4 UUID", sessionID)

	// Set cookie's age (10 minutes)
	seshAge := 600
	if r.FormValue("myMem") == "1" {
		seshAge = 34560000 // The maximum age of a cookie is 400 days
	}
	expiryTime := time.Now().Add(time.Duration(seshAge) * time.Second)

	// Save the session on the server side
	sessionMutex.Lock()
	sessions[sessionID.String()] = Sesh{expiryTime, r.FormValue("myName"), ""}
	sessionMutex.Unlock()

	// Send the session ID to the client as a Cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "session_id",
		Value:  sessionID.String(),
		Path:   "/",
		MaxAge: seshAge,
	})

	fmt.Printf("LoginHandler: %v (Sid: %v) - logged in!\n", sessions[sessionID.String()].Name, sessionID.String())

	p, err := template.ParseFiles("forum/info.html")
	if err != nil {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	w.WriteHeader(http.StatusOK)
	p.ExecuteTemplate(w, p.Name(), Result{"You have successfully logged in!", "", "Home"})
}

// Handler for users attempting to logout
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve session ID from Cookie - if either client or server side session doesn't exist, deny access
	cookie, err := r.Cookie("session_id")
	if err != nil {
		ErrorHandler(w, http.StatusUnauthorized, "You need to login to logout!", "", "Home")
		return
	}

	if _, ok := sessions[cookie.Value]; !ok {
		ErrorHandler(w, http.StatusForbidden, "You need to login to logout!", "", "Home")
		return
	}

	// Delete the session on server side
	fmt.Printf("LogoutHandler: %v (Sid: %v) - logged out\n", sessions[cookie.Value].Name, cookie.Value)
	sessionMutex.Lock()
	delete(sessions, cookie.Value)
	sessionMutex.Unlock()

	// Delete client cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "session_id",
		Value:  "",
		Path:   "/",
		MaxAge: -1, // Delete the Cookie
	})

	p, err := template.ParseFiles("forum/info.html")
	if err != nil {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	w.WriteHeader(http.StatusOK)
	p.ExecuteTemplate(w, p.Name(), Result{"You have successfully logged out.", "", "Home"})
}

// CLeans up expired sessions on the server side
func CleanExpiredSessions() {
	for {
		time.Sleep(1 * time.Second) // Run cleanup every second
		sessionMutex.Lock()
		for id, sesh := range sessions {
			if time.Now().After(sesh.Expiry) {
				fmt.Println(id, "- session expired")
				delete(sessions, id)
			}
		}
		sessionMutex.Unlock()
	}
}
