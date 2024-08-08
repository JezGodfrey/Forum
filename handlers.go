package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"text/template"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

type Index struct {
	Forum    []Stats
	Sessions []string
}

type Stats struct {
	Sub    int
	Name   string
	Topics int
	Posts  int
}

// General handler for index and non-specified pages
func Handler(w http.ResponseWriter, r *http.Request) {
	// Show and collect current user sessions
	var index Index
	fmt.Println("Current sessions:")
	sessionMutex.Lock()
	for id, sesh := range sessions {
		fmt.Println(id, sesh.Name)
		index.Sessions = append(index.Sessions, sesh.Name)
	}
	sessionMutex.Unlock()

	// For index page
	if r.URL.Path == "/" || r.URL.Path == "/index" || r.URL.Path == "/index.html" {
		// Gather stats for all subforums
		var stats Stats
		for i := 1; i <= CheckSubs(); i++ {
			stats.Sub = i
			s := strconv.Itoa(i)
			err := db.QueryRow("SELECT COALESCE((SELECT name FROM subforums WHERE id=?),0) as name, COUNT(sub) as ts, COALESCE(SUM(replies + 1),0) as ps FROM topics WHERE sub=?", s, s).Scan(&stats.Name, &stats.Topics, &stats.Posts)
			if err != nil {
				fmt.Println(err)
				ErrorHandler(w, http.StatusInternalServerError, "", "", "")
				return
			}
			index.Forum = append(index.Forum, stats)
		}

		slices.Sort(index.Sessions)

		// If file not found, display respective error page
		p, err := template.ParseFiles("forum/index.html")
		if err != nil {
			fmt.Println(err)
			ErrorHandler(w, http.StatusInternalServerError, "", "", "")
			return
		}

		w.WriteHeader(http.StatusOK)
		p.ExecuteTemplate(w, p.Name(), index)
		return
	} else {
		// Anything that isn't the index page has its own handler, so anything else is a 404
		ErrorHandler(w, http.StatusNotFound, "Page not found.", "", "Home")
	}
}

// Handler for the login page
func LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	// Logged in users can't access this page
	if CheckSession(w, r, true) {
		ErrorHandler(w, http.StatusForbidden, "403 Forbidden - You must log out to perform this action.", "", "Home")
		return
	}

	p, err := template.ParseFiles("forum/login.html")
	if err != nil {
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	w.WriteHeader(http.StatusOK)
	p.Execute(w, p.Name())
}

// Handler for registration page
func RegisterPageHandler(w http.ResponseWriter, r *http.Request) {
	// Logged in users can't access this page
	if CheckSession(w, r, true) {
		ErrorHandler(w, http.StatusForbidden, "403 Forbidden - You must log out to perform this action.", "", "Home")
		return
	}

	p, err := template.ParseFiles("forum/register.html")
	if err != nil {
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	w.WriteHeader(http.StatusOK)
	p.Execute(w, p.Name())
}

// Hashes the given password using bcrypt
func HashPassword(pass string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	return string(hash), err
}

// Handling registration requests
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	// Check for correct request method
	if r.Method != http.MethodPost {
		ErrorHandler(w, http.StatusMethodNotAllowed, "405 Method Not Allowed - Please submit the appropriate form.", "register.html", "Registration")
		return
	}

	// Check if input details match the specified formats
	email, _ := regexp.Compile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	format, _ := regexp.Compile(`^[\w]+$`)

	if !email.MatchString(r.FormValue("myMail")) {
		ErrorHandler(w, http.StatusBadRequest, "Invalid email address.", "register.html", "Registration")
		return
	}

	if !format.MatchString(r.FormValue("myName")) || (len(r.FormValue("myName")) < 1 || len(r.FormValue("myName")) > 20) || (len(r.FormValue("myPass")) < 4 || len(r.FormValue("myPass")) > 30) {
		ErrorHandler(w, http.StatusBadRequest, "Invalid username/password.", "register.html", "Registration")
		return
	}

	// Encrypt the password
	hash, err := HashPassword(r.FormValue("myPass"))
	if err != nil {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	// Check if email has already been used or username is taken
	var u string
	var e string
	err = db.QueryRow("SELECT email FROM users WHERE email = ?", r.FormValue("myMail")).Scan(&e)
	if err != nil && err != sql.ErrNoRows {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}
	if e == r.FormValue("myMail") {
		ErrorHandler(w, http.StatusBadRequest, "Email address already in use.", "register.html", "Registration")
		return
	}

	err = db.QueryRow("SELECT username FROM users WHERE username = ?", r.FormValue("myName")).Scan(&u)
	if err != nil && err != sql.ErrNoRows {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}
	if u == r.FormValue("myName") {
		ErrorHandler(w, http.StatusBadRequest, "Username taken.", "register.html", "Registration")
		return
	}

	// Insert details into database
	insert, err := db.Prepare("INSERT INTO users (email, username, pass) VALUES (?, ?, ?)")
	if err != nil {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}
	insert.Exec(r.FormValue("myMail"), r.FormValue("myName"), hash)
	insert.Close()

	p, err := template.ParseFiles("forum/info.html")
	if err != nil {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	fmt.Println("RegisterHandler: Registered user! -", r.FormValue("myName"))
	w.WriteHeader(http.StatusCreated)
	p.ExecuteTemplate(w, p.Name(), Result{"You have successfully registered!", "login.html", "Login"})
}
