package main

import (
	"database/sql"
	"log"
	"net/http"
)

// TO IMPROVE:
// - Refactor SQL queries to be performed by a singular query function
// - Refactor viewforum and viewthread pages to use the same template
// - Rework subforums implementation (see createdb.go line:69)

var db *sql.DB

func main() {
	// Open the database connection
	var err error
	db, err = sql.Open("sqlite3", "forumdb.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Test the connection
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	// Create tables and populate subforums
	CreateTables()
	CheckSubs()

	// Ensure static files from forum directory are served correctly
	fs := http.FileServer(http.Dir("./forum"))
	http.Handle("/forum/", http.StripPrefix("/forum/", fs))

	// Handling erroneous favicon.ico requests
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./forum/favicon.ico")
	})

	// Handlers for HTML/GET pages
	http.HandleFunc("/", Handler)
	http.HandleFunc("/register.html", RegisterPageHandler)
	http.HandleFunc("/login.html", LoginPageHandler)
	http.HandleFunc("/viewforum", ViewHandler)
	http.HandleFunc("/viewthread", ThreadHandler)
	http.HandleFunc("/posting", PostPageHandler)

	// POST handlers
	http.HandleFunc("/logging", LoginHandler)
	http.HandleFunc("/logout", LogoutHandler)
	http.HandleFunc("/registration", RegisterHandler)
	http.HandleFunc("/post", PostHandler)
	http.HandleFunc("/like/", LikeHandler)
	http.HandleFunc("/dislike/", LikeHandler)

	// Go routine to continuously check for expired sessions
	go CleanExpiredSessions()

	// Run the server
	log.Fatal(http.ListenAndServe(":8080", nil))
}
