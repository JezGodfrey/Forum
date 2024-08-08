package main

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"
)

type View struct {
	Mode    string
	Sub     string
	SubName string
	Name    string
	Tid     string
	Pid     string
	Filters []Filter
	Posts   []Post
	Reply   string
	Pages   []Num
	Count   int
}

type Post struct {
	Tid        int
	Pid        int
	Title      string
	Table      string
	Author     string
	Date       string
	Categories string
	Content    string
	Likes      int
	Dislikes   int
}

type Num int

// Parse the URL to access query attributes in GET request
func RequestParse(r *http.Request) url.Values {
	parsedURL, err := url.Parse(r.URL.String())
	if err != nil {
		fmt.Println("Error parsing URL:", err)
		return nil
	}

	return parsedURL.Query()
}

// Viewing subforums, displaying filters and topics
func ViewHandler(w http.ResponseWriter, r *http.Request) {
	// Get any URL attributes
	query := RequestParse(r)
	if query == nil {
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}
	sub := query.Get("sub")
	cat := query.Get("cat")
	start := query.Get("start")

	// Check if requested subforum exists
	var subName string
	err := db.QueryRow("SELECT name FROM subforums WHERE id=?", sub).Scan(&subName)
	if err != nil {
		if err == sql.ErrNoRows {
			ErrorHandler(w, http.StatusNotFound, "Requested subforum doesn't exist.", "", "Home")
		}
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	var view View
	view.Sub = sub
	subNum, _ := strconv.Atoi(sub)
	view.SubName = subName
	view.Filters = forumFilters[subNum-1]

	// Calculating which topics to view based on page number requested
	startNum, err := strconv.Atoi(start)
	if start == "" {
		startNum = 0
	} else {
		if err != nil || startNum%30 != 0 {
			fmt.Println(err)
			ErrorHandler(w, http.StatusBadRequest, "400 Bad Request - Invalid subforum page number.", fmt.Sprintf("viewforum?sub=%v", sub), subName)
			return
		}
	}

	// Filter management - different SQL queries needed depending on filters chosen
	var sqlQuery string
	switch cat {
	case "USER":
		// Only accessible by logged in users
		if !CheckSession(w, r, false) {
			return
		}

		cookie, _ := r.Cookie("session_id")
		sqlQuery = "SELECT t.id, t.title, t.author, t.date, t.category, t.replies, (SELECT id FROM posts WHERE tid=t.id LIMIT 1) as pid, COALESCE((SELECT COUNT(liked) from liked_posts WHERE tid=t.id AND pid=(SELECT id FROM posts WHERE tid=t.id LIMIT 1) AND liked=1),0) as likes, COALESCE((SELECT COUNT(disliked) from liked_posts WHERE tid=t.id AND pid=(SELECT id FROM posts WHERE tid=t.id LIMIT 1) AND disliked=1),0) as dislikes FROM topics t WHERE sub=? AND t.author=\"" + sessions[cookie.Value].Name + "\" ORDER BY lastupdated DESC"
	case "LIKE":
		// Only accessible by logged in users
		if !CheckSession(w, r, false) {
			return
		}

		cookie, _ := r.Cookie("session_id")
		sqlQuery = "SELECT t.id, t.title, t.author, t.date, t.category, t.replies, (SELECT id FROM posts WHERE tid=t.id LIMIT 1) as pid, COALESCE((SELECT COUNT(liked) from liked_posts WHERE tid=t.id AND pid=(SELECT id FROM posts WHERE tid=t.id LIMIT 1) AND liked=1),0) as likes, COALESCE((SELECT COUNT(disliked) from liked_posts WHERE tid=t.id AND pid=(SELECT id FROM posts WHERE tid=t.id LIMIT 1) AND disliked=1),0) as dislikes FROM topics t INNER JOIN liked_posts ON t.id=liked_posts.tid WHERE sub=? AND pid=(SELECT id FROM posts WHERE tid=t.id LIMIT 1) AND liked=1 AND user=\"" + sessions[cookie.Value].Name + "\" ORDER BY lastupdated DESC"
	default:
		// General query to match filter value (see forumFilters variable in createdb.go)
		sqlQuery = "SELECT t.id, t.title, t.author, t.date, t.category, t.replies, (SELECT id FROM posts WHERE tid=t.id LIMIT 1) as pid, COALESCE((SELECT COUNT(liked) from liked_posts WHERE tid=t.id AND pid=(SELECT id FROM posts WHERE tid=t.id LIMIT 1) AND liked=1),0) as likes, COALESCE((SELECT COUNT(disliked) from liked_posts WHERE tid=t.id AND pid=(SELECT id FROM posts WHERE tid=t.id LIMIT 1) AND disliked=1),0) as dislikes FROM topics t WHERE sub=? AND category LIKE '%" + cat + "%' ORDER BY lastupdated DESC"
	}

	// Getting topic information
	var topic Post
	rows, err := db.Query(sqlQuery, sub)
	if err != nil && err != sql.ErrNoRows {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}
	for rows.Next() {
		rows.Scan(&topic.Tid, &topic.Title, &topic.Author, &topic.Date, &topic.Categories, &topic.Content, &topic.Pid, &topic.Likes, &topic.Dislikes)
		view.Posts = append(view.Posts, topic)
	}

	// Calculating number of topics and resulting page numbers
	view.Count, view.Pages = countPosts(len(view.Posts), 30)
	if startNum > view.Count {
		startNum = 0
	}
	toEnd := startNum + 30
	if startNum+30 > view.Count {
		toEnd = view.Count
	}
	view.Posts = view.Posts[startNum:toEnd]

	p, err := template.ParseFiles("forum/view.html")
	if err != nil {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	w.WriteHeader(http.StatusOK)
	err = p.ExecuteTemplate(w, p.Name(), view)
	if err != nil {
		fmt.Println(err)
	}
}

// Receivers to append and alter the start attribute for GET requests corresponding to page numbers
func (n Num) StartPost() string {
	return fmt.Sprintf("&start=%v", (int(n)-1)*10)
}

func (n Num) StartTopic() string {
	return fmt.Sprintf("&start=%v", (int(n)-1)*30)
}

// Count the number of posts to determine how many pages there should be (10 per page)
func countPosts(count, limit int) (int, []Num) {
	var pages []Num
	pagenums := int(math.Ceil(float64(count) / float64(limit)))
	for i := 1; i < pagenums+1; i++ {
		pages = append(pages, Num(i))
	}

	return count, pages
}

// Viewing topic threads
func ThreadHandler(w http.ResponseWriter, r *http.Request) {
	// Check subforum exists
	query := RequestParse(r)
	sub := query.Get("sub")
	var subName string
	err := db.QueryRow("SELECT name FROM subforums WHERE id=?", sub).Scan(&subName)
	if err != nil {
		if err == sql.ErrNoRows {
			ErrorHandler(w, http.StatusNotFound, "Requested subforum doesn't exist.", "", "Home")
		}
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	var thread View
	thread.Sub = sub
	thread.SubName = subName
	thread.Tid = query.Get("t")

	// Get topic details
	var trueSub string
	err = db.QueryRow("SELECT title, sub FROM topics WHERE id=?", thread.Tid).Scan(&thread.Name, &trueSub)
	if err != nil {
		if err == sql.ErrNoRows {
			ErrorHandler(w, http.StatusNotFound, "Requested thread does not exist", "", "Home")
			return
		}
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	// Ensure the thread is within the requested subforum
	if sub != trueSub {
		ErrorHandler(w, http.StatusNotFound, "Requested thread does not exist.", "", "Home")
		return
	}

	// Determining which posts to show based on page requested
	start := query.Get("start")
	startNum, err := strconv.Atoi(start)
	if start == "" {
		startNum = 0
	} else {
		if err != nil || startNum%10 != 0 {
			fmt.Println(err)
			ErrorHandler(w, http.StatusBadRequest, "400 Bad Request - Invalid thread page number.", fmt.Sprintf("viewthread?t=%v&sub=%v", thread.Tid, sub), "Thread")
			return
		}
	}

	// Get details of posts
	var post Post
	rows, err := db.Query("SELECT id, author, subject, date, content, COALESCE((SELECT COUNT(liked) FROM liked_posts WHERE liked=1 AND pid=id AND tid=?),0) as likes, COALESCE((SELECT COUNT(disliked) FROM liked_posts WHERE disliked=1 AND pid=id AND tid=?),0) as dislikes FROM posts WHERE tid=?", thread.Tid, thread.Tid, thread.Tid)
	if err != nil {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}
	for rows.Next() {
		rows.Scan(&post.Pid, &post.Author, &post.Title, &post.Date, &post.Content, &post.Likes, &post.Dislikes)
		thread.Posts = append(thread.Posts, post)
	}

	// Calculating number of topics and resulting page numbers
	thread.Count, thread.Pages = countPosts(len(thread.Posts), 10)
	if startNum > thread.Count {
		startNum = 0
	}
	toEnd := startNum + 10
	if startNum+10 > thread.Count {
		toEnd = thread.Count
	}
	thread.Posts = thread.Posts[startNum:toEnd]

	p, err := template.ParseFiles("forum/thread.html")
	if err != nil {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	w.WriteHeader(http.StatusOK)
	p.ExecuteTemplate(w, p.Name(), thread)
}

// Handler for creating posts, both topics and replies
func PostPageHandler(w http.ResponseWriter, r *http.Request) {
	// Only logged in users have access
	if !CheckSession(w, r, false) {
		return
	}

	// Check if subforum exists
	query := RequestParse(r)
	sub := query.Get("sub")
	var subName string
	err := db.QueryRow("SELECT name FROM subforums WHERE id=?", sub).Scan(&subName)
	if err != nil {
		if err == sql.ErrNoRows {
			ErrorHandler(w, http.StatusNotFound, "Requested subforum doesn't exist.", "", "Home")
			return
		}
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	var view View
	view.Sub = sub
	subNum, _ := strconv.Atoi(sub)
	view.SubName = subName
	view.Filters = forumFilters[subNum-1]
	mode := query.Get("mode")
	view.Mode = mode

	// Respond differently for if it's a new reply to an existing thread
	if mode == "reply" {
		// Check topic exists
		var tname string
		var trueSub string
		t := query.Get("t")
		err := db.QueryRow("SELECT title, sub FROM topics WHERE id=?", t).Scan(&tname, &trueSub)
		if err != nil {
			if err == sql.ErrNoRows {
				ErrorHandler(w, http.StatusNotFound, "Requested thread does not exist", fmt.Sprintf("viewforum?sub=%v", sub), subName)
				return
			}
			fmt.Println(err)
			ErrorHandler(w, http.StatusInternalServerError, "", "", "")
			return
		}

		// If it does, ensure the correct sub given in request
		if sub != trueSub {
			ErrorHandler(w, http.StatusBadRequest, "Requested thread not found within this subforum.", fmt.Sprintf("posting?t=%v&sub=%v&mode=reply", t, trueSub), "Post Reply")
			return

		}

		view.Reply = "Re: " + tname
		view.Tid = t
	}

	p, err := template.ParseFiles("forum/posting.html")
	if err != nil {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	w.WriteHeader(http.StatusOK)
	p.ExecuteTemplate(w, p.Name(), view)
}

// Get the categories selected by the user when creating a post
func getCats(r *http.Request, fs []Filter) string {
	var s string
	for _, f := range fs {
		if r.FormValue(f.Value) != "" {
			s = s + r.FormValue(f.Value) + "|"
		}
	}

	if s == "" {
		return s
	}

	return s[:len(s)-1]
}

// Submitted posts to be posted
func PostHandler(w http.ResponseWriter, r *http.Request) {
	// Check for correct request method
	if r.Method != http.MethodPost {
		ErrorHandler(w, http.StatusMethodNotAllowed, "405 Method Not Allowed - Please submit the appropriate form.", "", "Home")
		return
	}

	if !CheckSession(w, r, false) {
		return
	}

	cookie, _ := r.Cookie("session_id")
	sub, _ := strconv.Atoi(r.FormValue("topSub"))
	mode := r.FormValue("topMode")
	datetime := time.Now().Format("Mon Jan 02, 2006 15:04 pm")
	epoch := time.Now().UnixMilli()

	// Behave differently according to whether a new topic or new reply to existing topic was posted
	if mode == "topic" {
		// Ensuring the post isn't empty/invalid
		topicname := r.FormValue("topName")
		if len(topicname) < 1 || len(r.FormValue("topContent")) < 1 {
			ErrorHandler(w, http.StatusBadRequest, "Invalid post.", fmt.Sprintf("posting?sub=%v&mode=%v", sub, mode), "New Topic")
			return
		}
		if len(topicname) > 200 {
			topicname = topicname[:200]
		}

		// Check if topic already exists
		var topicExists string
		db.QueryRow("SELECT title FROM topics WHERE title=?", topicname).Scan(&topicExists)
		if topicExists != "" {
			ErrorHandler(w, http.StatusBadRequest, "Topic already exists.", fmt.Sprintf("posting?sub=%v&mode=%v", sub, mode), "New Topic")
			return
		}

		// Insert topic into topics table
		cats := getCats(r, forumFilters[sub-1])
		sqlQuery := "INSERT INTO topics (title, date, sub, category, author, lastupdated) VALUES (?, ?, ?, ?, ?, ?)"
		insert, err := db.Prepare(sqlQuery)
		if err != nil {
			fmt.Println(err)
			ErrorHandler(w, http.StatusInternalServerError, "", "", "")
			return
		}
		insert.Exec(topicname, datetime, r.FormValue("topSub"), cats, sessions[cookie.Value].Name, epoch)
		insert.Close()

		// Get the id of newly inserted topic
		var tid string
		db.QueryRow("SELECT id FROM topics WHERE title=?", topicname).Scan(&tid)

		// and add the first post
		sqlQuery = "INSERT INTO posts (tid, author, subject, date, content) VALUES (?, ?, ?, ?, ?)"
		insert, err = db.Prepare(sqlQuery)
		if err != nil {
			fmt.Println(err)
			ErrorHandler(w, http.StatusInternalServerError, "", "", "")
			return
		}
		insert.Exec(tid, sessions[cookie.Value].Name, topicname, datetime, r.FormValue("topContent"))
		insert.Close()

		p, err := template.ParseFiles("forum/info.html")
		if err != nil {
			fmt.Println(err)
			ErrorHandler(w, http.StatusInternalServerError, "", "", "")
			return
		}

		w.WriteHeader(http.StatusCreated)
		p.ExecuteTemplate(w, p.Name(), Result{"Topic successfully posted!", fmt.Sprintf("viewthread?t=%v&sub=%v", tid, sub), topicname})
		return
	}

	if mode == "reply" {
		// Ensuring the post isn't empty/invalid
		if len(r.FormValue("topName")) < 1 || len(r.FormValue("topContent")) < 1 {
			ErrorHandler(w, http.StatusBadRequest, "Invalid post.", fmt.Sprintf("viewforum?sub=%v", sub), "Subforum")
			return
		}

		// Add post to database
		sqlQuery := "INSERT INTO posts (tid, author, subject, date, content) VALUES (?, ?, ?, ?, ?)"
		insert, err := db.Prepare(sqlQuery)
		if err != nil {
			fmt.Println(err)
			ErrorHandler(w, http.StatusInternalServerError, "", "", "")
			return
		}
		insert.Exec(r.FormValue("topTid"), sessions[cookie.Value].Name, r.FormValue("topName"), datetime, r.FormValue("topContent"))
		insert.Close()

		// Update the topics table to add one more to its 'replies' column
		sqlQuery = "UPDATE topics SET replies = replies + 1, lastupdated = ? WHERE id=?"
		insert, err = db.Prepare(sqlQuery)
		if err != nil {
			fmt.Println(err)
			ErrorHandler(w, http.StatusInternalServerError, "", "", "")
			return
		}
		insert.Exec(epoch, r.FormValue("topTid"))
		insert.Close()

		p, err := template.ParseFiles("forum/info.html")
		if err != nil {
			fmt.Println(err)
			ErrorHandler(w, http.StatusInternalServerError, "", "", "")
			return
		}

		w.WriteHeader(http.StatusCreated)
		p.ExecuteTemplate(w, p.Name(), Result{"Reply successfully posted!", fmt.Sprintf("viewthread?t=%v&sub=%v", r.FormValue("topTid"), sub), "Thread"})
	} else {
		// If post mode is not for a new topic or new reply, then bad request
		ErrorHandler(w, http.StatusBadRequest, "400 Bad Request - Incorrect post mode specified.", "", "Home")
	}
}

// Updating like/dislike counts
func LikeHandler(w http.ResponseWriter, r *http.Request) {
	// Check for correct request method
	if r.Method != http.MethodPost {
		ErrorHandler(w, http.StatusMethodNotAllowed, "405 Method Not Allowed - I 'dislike' what you did there.", "", "Home")
		return
	}

	// Only logged in users can like posts
	if !CheckSession(w, r, true) {
		return
	}

	cookie, _ := r.Cookie("session_id")
	path := strings.Split(r.URL.Path, "/")

	// Check if the user has already liked or disliked the target post
	var liked bool
	var disliked bool
	err := db.QueryRow("SELECT liked, disliked FROM liked_posts WHERE tid=? AND pid=? AND user=?", path[2], path[3], sessions[cookie.Value].Name).Scan(&liked, &disliked)
	if err != nil && err != sql.ErrNoRows {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	// If the user hasn't, insert their (dis)like
	if err == sql.ErrNoRows {
		sqlQuery := "INSERT INTO liked_posts (tid, pid, user, " + path[1] + "d) VALUES (?, ?, ?, true)"
		insert, err := db.Prepare(sqlQuery)
		if err != nil {
			fmt.Println(err)
			ErrorHandler(w, http.StatusInternalServerError, "", "", "")
			return
		}
		insert.Exec(path[2], path[3], sessions[cookie.Value].Name)
		insert.Close()
	}

	// If the user has, alter the database accordingly (both liked and disliked columns can't be true at the same time)
	if err == nil {
		if liked && path[1] == "like" {
			sqlQuery := "DELETE FROM liked_posts WHERE tid=? AND pid=? AND user=?"
			insert, err := db.Prepare(sqlQuery)
			if err != nil {
				fmt.Println(err)
				ErrorHandler(w, http.StatusInternalServerError, "", "", "")
				return
			}
			insert.Exec(path[2], path[3], sessions[cookie.Value].Name)
			insert.Close()
		}

		if disliked && path[1] == "dislike" {
			sqlQuery := "DELETE FROM liked_posts WHERE tid=? AND pid=? AND user=?"
			insert, err := db.Prepare(sqlQuery)
			if err != nil {
				fmt.Println(err)
				ErrorHandler(w, http.StatusInternalServerError, "", "", "")
				return
			}
			insert.Exec(path[2], path[3], sessions[cookie.Value].Name)
			insert.Close()
		}

		if liked && path[1] == "dislike" {
			sqlQuery := "UPDATE liked_posts SET liked = 0, disliked = 1 WHERE tid=? AND pid=? AND user=?"
			insert, err := db.Prepare(sqlQuery)
			if err != nil {
				fmt.Println(err)
				ErrorHandler(w, http.StatusInternalServerError, "", "", "")
				return
			}
			insert.Exec(path[2], path[3], sessions[cookie.Value].Name)
			insert.Close()
		}

		if disliked && path[1] == "like" {
			sqlQuery := "UPDATE liked_posts SET liked = 1, disliked = 0 WHERE tid=? AND pid=? AND user=?"
			insert, err := db.Prepare(sqlQuery)
			if err != nil {
				fmt.Println(err)
				ErrorHandler(w, http.StatusInternalServerError, "", "", "")
				return
			}
			insert.Exec(path[2], path[3], sessions[cookie.Value].Name)
			insert.Close()
		}
	}

	// Obtain the new number of likes/dislikes for the target post
	var likes string
	var dislikes string
	err = db.QueryRow("SELECT COUNT(*) FROM liked_posts WHERE tid=? AND pid=? AND liked=1", path[2], path[3]).Scan(&likes)
	if err != nil {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}
	err = db.QueryRow("SELECT COUNT(*) FROM liked_posts WHERE tid=? AND pid=? AND disliked=1", path[2], path[3]).Scan(&dislikes)
	if err != nil {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	// Send the new number of likes/dislikes in json format and update via javascript
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"id":` + path[3] + `,"likes":` + likes + `,"dislikes":` + dislikes + `}`))
}
