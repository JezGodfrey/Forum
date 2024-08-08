package main

import (
	"database/sql"
	"log"
)

// Create tables if they don't exist
func CreateTables() {
	createTables := []string{`CREATE TABLE IF NOT EXISTS "users" (
	"id"	INTEGER NOT NULL UNIQUE,
	"username"	TEXT NOT NULL UNIQUE,
	"pass"	TEXT NOT NULL,
	"email"	INTEGER NOT NULL UNIQUE,
	PRIMARY KEY("id")
	)`,
		`CREATE TABLE IF NOT EXISTS "subforums" (
	"id"	INTEGER NOT NULL UNIQUE,
	"name"	TEXT NOT NULL UNIQUE,
	PRIMARY KEY("id")
	)`,
		`CREATE TABLE IF NOT EXISTS "topics" (
	"id"	INTEGER NOT NULL UNIQUE,
	"title"	TEXT NOT NULL,
	"author"	TEXT NOT NULL,
	"date"	TEXT NOT NULL,
	"sub"	INTEGER NOT NULL DEFAULT 1,
	"category"	TEXT,
	"replies"	INTEGER NOT NULL DEFAULT 0,
	"lastupdated"	TEXT NOT NULL,
	PRIMARY KEY("id")
	)`,
		`CREATE TABLE IF NOT EXISTS "posts" (
	"id"	INTEGER NOT NULL UNIQUE,
	"tid"	INTEGER NOT NULL,
	"author"	INTEGER NOT NULL,
	"subject"	INTEGER NOT NULL,
	"date"	INTEGER NOT NULL,
	"content"	INTEGER NOT NULL,
	FOREIGN KEY("tid") REFERENCES "topics"("id"),
	PRIMARY KEY("id")
	)`,
		`CREATE TABLE IF NOT EXISTS "liked_posts" (
	"tid"	INTEGER,
	"pid"	INTEGER,
	"user"	TEXT,
	"liked"	INTEGER DEFAULT 0,
	"disliked"	INTEGER DEFAULT 0,
	FOREIGN KEY("tid") REFERENCES "topics"("id"),
	FOREIGN KEY("pid") REFERENCES "posts"("id")
	)`}

	// Prepare and execute each query above
	for _, table := range createTables {
		query, err := db.Prepare(table)
		if err != nil {
			log.Fatal(err)
		}

		_, err = query.Exec()
		if err != nil && err != sql.ErrNoRows {
			log.Fatal(err)
		}

		query.Close()
	}
}

// This forum was initially designed to only have 2 subforums. This part of the forum is not currently easily scalable.
// To add another subforum, add another INSERT query to the subQueries variable, and add another set of filters (can be empty) to the forumFilters variable
type Filter struct {
	Name  string
	Value string
}

var forumFilters = [][]Filter{
	{{"Switch", "NSW"}, {"PS5", "PS5"}, {"Xbox Series X", "XSX"}, {"PC", "PC"}, {"Retro", "Retro"}, {"Other", "Other"}},
	{{"Entertainment", "Entertainment"}, {"Sports", "Sports"}, {"Politics", "Pol"}, {"Other", "Other"}}}

//, {}}

func CheckSubs() int {
	subQueries := []string{
		`INSERT INTO subforums (Name) VALUES("Games")`,
		`INSERT INTO subforums (Name) VALUES("Off-Topic")`}
	// , INSERT INTO subforums (Name) Values("New Sub")}

	// Count how many subforums are currently in the database
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM subforums").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	// If the number of subforums in database don't match the number of insert queries above, add the new subforum(s)
	if count != len(subQueries) {
		for i := count; i < len(subQueries); i++ {
			query, err := db.Prepare(subQueries[i])
			if err != nil {
				log.Fatal(err)
			}

			_, err = query.Exec()
			if err != nil && err != sql.ErrNoRows {
				log.Fatal(err)
			}

			query.Close()
			count++
		}
	}

	return count
}
