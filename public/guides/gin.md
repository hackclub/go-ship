# Gin Web Framework

<img src="/public/guides/assets/gin.webp" alt="Gin Logo" width="250">

Gin is a web framework made for websites/APIs in Go. It's designed to be fast, simple, and easy to use. A few great
features it offers are middleware support, crash recovery, routing+grouping, and more.

You can find the documentation for Gin at <https://gin-gonic.com/en/>.

For this guide, we'll be using Gin to build a simple public message board using Gin, SQLite and templates.

## Getting Started

Assuming you already have Go installed, and are inside a folder, let's initialize a new Go module:

```bash
go mod init website.com/yourusername/yourproject
```

Obviously, replace `website.com` with the site where your git repo is hosted (e.g. `codeberg.org`, `github.com`), and `yourusername/yourproject` with your username and project name. This is important for importing packages later on.

You can also just do `go mod init yourproject`, but using a full module path is a good practice.

Next, let's install Gin:

```bash
go get -u github.com/gin-gonic/gin
```

Now, let's get to the `main.go` file.

---

We'll start off by making a basic Gin server that returns "Hello World" when you visit the homepage:

```go
package main

import "github.com/gin-gonic/gin"

func main() {
	// gin.Default initializes a new Gin router with default middleware (logger and recovery)
	r := gin.Default()

	// r.GET defines a route for the HTTP GET method at the root path ("/").
	// When this route is accessed, it executes the provided handler function,
	// in this case, returning a JSON response with a message "Hello, World!" and an HTTP status code of 200 (OK).
	r.GET("/", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"message": "Hello, World!"})
	})

	// r.Run() starts the HTTP server on the defined address (default is ":8080")
	r.Run(":8080")
}
```

You can now run this server with `go run main.go` and visit <http://localhost:8080> to see the "Hello, World!" message.

![Hello World](/public/guides/assets/gin-hello.webp)

You might get warnings about debug mode and trusted proxies, but you can ignore those while developing locally. You can read more about those in the documentation:

- Trusted Proxies: <https://gin-gonic.com/en/docs/server-config/trusted-proxies/>
- Debug Mode: <https://gin-gonic.com/en/docs/faq/#how-do-i-run-gin-in-production-mode>

---

But just a "Hello World" isn't enough. Let's make it display HTML, using templates.

Gin has built-in support for Go's `html/template` package, which allows us to render HTML templates easily.

First, let's create a folder called `templates` in the root directory. Inside that folder, create a file called `index.html` with the following content:

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>{{ .Title }}</title>
  </head>
  <body>
    {{ .Title }}
  </body>
</html>
```

The `{{ .Title }}` syntax is the template syntax for inserting data. In this case, it will insert the value of `Title` that we pass from our Go code.

You can read more about Go's HTML templating syntax in the official documentation: <https://pkg.go.dev/html/template>.

Now, let's modify our `main.go` to render this template instead of returning JSON:

```go
package main

import (
	"github.com/gin-gonic/gin"
)

func main() {
	// gin.Default initializes a new Gin router with default middleware (logger and recovery)
	r := gin.Default()

	// Load HTML Templates into the Gin router.
	r.LoadHTMLGlob("templates/*")

	// r.GET defines a route for the HTTP GET method at the root path ("/").
	// When this route is accessed, it executes the provided handler function,
	// in this case, rendering the "index.html" template with a title variable.
	r.GET("/", func(ctx *gin.Context) {
		ctx.HTML(200, "index.html", gin.H{
			"Title": "Go Public Board",
		})
	})

	// r.Run() starts the HTTP server on the defined address (default is ":8080")
	r.Run(":8080")
}
```

Now, when you run the server and visit <http://localhost:8080>, you should see "Go Public Board" displayed on the page, rendered from the template.

<img src="/public/guides/assets/gin-title.webp" alt="Gin Template" width="400">

---

But how do we get CSS? By serving static files! Using r.Static, we can serve static files from a folder. Let's create a folder called `public` in the root directory, and inside that folder, create a file called `style.css` with the following content:

```css
@import url("https://fonts.googleapis.com/css2?family=Cause:wght@100..900&display=swap");

body {
  font-family: "Cause", sans-serif;
  background-color: #f0f0f0;
  margin: 0;
  padding: 20px;
}
```

Now, let's modify our `main.go` to serve this static file.

Add this line anywhere before `r.Run()`:

```go
// r.Static serves static files from the specified directory.
r.Static("/public", "./public")
```

Now, you can link to this CSS file in your `index.html` like this:

```html
<link rel="stylesheet" href="/public/style.css" />
```

Now, when you refresh the page, you should see the new styling applied!

---

Alright, now we have static files and a basic template. But how do we actually get users to post messages? By making a HTML form, and adding a POST route to handle the form submission.

Let's add the following to our index.html:

```html
<form action="/post" method="POST">
  <input type="text" name="author" placeholder="Username..." required />
  <input type="text" name="content" placeholder="Write something..." required />
  <button type="submit">Post</button>
</form>
```

And let's add the connecting POST route to our `main.go`:

```go
// r.POST defines a route for the HTTP POST method.
// This will be handling form submissions to create a new post on the board.
r.POST("/post", func(ctx *gin.Context) {
})
```

But hold on, how do we actually store these posts? For that, we'll be using a database. In this guide, we'll be using SQLite, which is a simple lightweight file-based database.

To use SQLite in Go, we can use the `github.com/mattn/go-sqlite3` package. Let's install it:

```bash
go get -u github.com/mattn/go-sqlite3
```

Note: `github.com/mattn/go-sqlite3` uses CGO, so you may need a C compiler installed. On Linux/macOS, you 90% likely already have one. On Windows, you can install MinGW or use WSL (Windows Subsystem for Linux).

To keep main.go clean, let's make a new file called `db.go` in a new directory `db`, which will handle the database.

```go
package db

import (
	"database/sql"
	"fmt"

	// _ is used to import the SQLite3 driver without directly referencing it in the code, preventing "imported and not used" errors.
	_ "github.com/mattn/go-sqlite3"
)

// DB is a global variable that holds the database connection pool, used for executing queries.
var DB *sql.DB

// InitDB initializes the database connection and creates the necessary tables if they do not exist.
func InitDB() {
	var err error
	// sql.Open opens a connection to the SQLite3 database file "public_board.db", creating it if it does not exist.
	DB, err = sql.Open("sqlite3", "public_board.db")
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to database: %v", err))
	}

	// Create the "posts" table if it does not exist, with columns for id, content, and created_at timestamp.
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		author TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err = DB.Exec(createTableQuery)
	if err != nil {
		panic(fmt.Sprintf("Failed to create posts table: %v", err))
	}
}
```

Then, in our `main.go`, we can initialize the database before the app starts, using `func init()`, which is a special function in Go that runs before the main function:

```go
func init() {
    db.InitDB()
}
```

Usually, the IDE should automatically import the `db` package for you, but if it doesn't, just add this to the top of your `main.go`:

```go
import "website.org/yourusername/yourproject/db"
```

Now, we can use `db.DB` to interact with the database in our POST route:

```go
// r.POST defines a route for the HTTP POST method.
// This will be handling form submissions to create a new post on the board.
r.POST("/post", func(ctx *gin.Context) {
    // Let's do some basic validation, and limits.

    // ctx.PostForm retrieves form data sent in the POST request.
    author := ctx.PostForm("author")
    content := ctx.PostForm("content")

    // Trim whitespace from the author and content to prevent posts that are just spaces.
    author = strings.TrimSpace(author)
    content = strings.TrimSpace(content)

    // Basic validation to ensure author and content are not empty.
    if author == "" || content == "" {
        ctx.JSON(400, gin.H{"error": "Author and content cannot be empty"})
        return
    }

    // And length check.
    if len(author) > 32 || len(content) > 300 {
        ctx.JSON(400, gin.H{"error": "Author name or content is too long"})
        return
    }

    // Let's add the post to the database.
    // Using ? placeholders in the SQL query to prevent SQL injection, and passing the author and content as parameters.
    _, err := db.DB.Exec("INSERT INTO posts (author, content) VALUES (?, ?)", author, content)
    if err != nil {
        ctx.JSON(500, gin.H{"error": "Failed to save post"})
        return
    }

    // And finally, we can redirect back to the main page after successfully adding the post.
    ctx.Redirect(303, "/")
})
```

Now, when you submit the form, it will add the post to the database. But we still need to display the posts on the main page.

To do that, we'll be making a type struct to represent a post, and then we'll query the database for all posts, scan them into a variable, and pass that variable to the template.

```go
type Post struct {
    ID        int
    Author    string
    Content   string
    CreatedAt string
}

r.GET("/", func(ctx *gin.Context) {
    // Fetch posts from the database to display on the main page.
    rows, err := db.DB.Query("SELECT id, author, content, created_at FROM posts ORDER BY created_at DESC")
    if err != nil {
        ctx.JSON(500, gin.H{"error": "Failed to fetch posts"})
        return
    }
    // Don't forget to defer closing the rows to prevent memory leaks.
    // Defer ensures that the function (here, rows.Close()) will be called after
    // the surrounding function (the handler) returns, even if an error occurs.
    defer rows.Close()

    var posts []Post
    // Iterate over the rows returned by the query and scan them into Post structs.
    for rows.Next() {
        var post Post
        err := rows.Scan(&post.ID, &post.Author, &post.Content, &post.CreatedAt)
        if err != nil {
            ctx.JSON(500, gin.H{"error": "Failed to parse posts"})
            return
        }
        posts = append(posts, post)
    }
    if err = rows.Err(); err != nil {
        ctx.JSON(500, gin.H{"error": "Error iterating over posts"})
        return
    }

    // Render the "index.html" template, passing the title and the list of posts.
    ctx.HTML(200, "index.html", gin.H{
        "Title": "Go Public Board",
        "Posts": posts,
    })
})
```

And then, in our `index.html`, we can display the posts like this:

```html
<!-- range iterates over every single entry in the Posts variable -->
{{ range .Posts }}
<!-- .Author, .Content, and .CreatedAt are the fields of the Post struct -->
<div>
  <h3>{{ .Author }}</h3>
  <p>{{ .Content }}</p>
  <small>{{ .CreatedAt }}</small>
</div>
{{ end }}
```

Now, when you submit a post, it will be saved to the database and displayed on the main page!

Cool thing about templates, is that they prevent XSS by default, so if you try to submit a post with HTML or JavaScript, it will be escaped and displayed as text instead of being executed! (As long as you don't use unsafe template options.)

<img src="/public/guides/assets/gin-form.webp" alt="Gin Board" width="400">

---

Pretty cool, right? This is just a basic example of what you can do with Gin. Maybe you could add these features next:

- Editing and deleting posts
- API endpoint for a random post
- User accounts and authentication
- Pagination for posts
- Sorts/Search for posts

You can find the full code for this guide on <https://codeberg.org/BananaJeans/go-public-board>.
