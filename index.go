package main

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"fmt"
	"golang.org/x/net/html"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"net/url"
	"os"
	"strconv"
)

var db, dbError = sql.Open("sqlite3", "./database.db")
const statusProcessing = 0
const statusDone = 1

func main() {

	db.Ping()

	checkError(dbError)

	defer db.Close()

	/*r := gin.Default()
	r.GET("/", index)
	r.GET("/parse", parse)
	r.Run(":8080")*/
	parse()
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func index(c *gin.Context) {
	c.JSON(200, gin.H{"message": "Hey"})
}

func parse() {
//func parse(c *gin.Context) {
	link := "http://fie.org/fie/structure/manufacturers"
	link = "http://www.zaryachem.com/ru"

	// insert

	u, err := url.Parse(link)
	checkError(err)

	id := insertUrl(u.String())

	grab(u, id)

	fmt.Printf("\n%v\n\n", id)

	//c.JSON(200, gin.H{"message": "processing", "jobId": id})
}

func insertUrl(url string) int64 {
	stmt, err := db.Prepare("INSERT INTO url(url, status) values(?, ?)")
	checkError(err)
	res, err := stmt.Exec(url, statusProcessing)
	checkError(err)
	id, err := res.LastInsertId()
	checkError(err)

	return id
}

func insertImageUrl(url string, urlId int64) int64 {
	stmt, err := db.Prepare("INSERT INTO image(url, url_id) values(?, ?)")
	checkError(err)
	res, err := stmt.Exec(url, urlId)
	checkError(err)
	id, err := res.LastInsertId()
	checkError(err)

	return id
}

func grab(u *url.URL, id int64) {
	resp, err := http.Get(u.String())
	checkError(err)

	doc, err := html.Parse(resp.Body)
	checkError(err)

	getImages(doc, u, id)
}

func getImages(n *html.Node, u *url.URL, id int64) {
	if n.Type == html.ElementNode && n.Data == "img" {
		for i := 0; i < len(n.Attr); i++ {
			if n.Attr[i].Key == "src" {
				downloadImage(n.Attr[i].Val, u, id)
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		getImages(c, u, id)
	}
}

func downloadImage(image string, u *url.URL, id int64) {

	imageLink := createImageLink(image, u)

	fmt.Println(imageLink)

	resp, err := http.Get(imageLink)
	checkError(err)

	imageId := insertImageUrl(imageLink, id)

	imageBody, err := ioutil.ReadAll(resp.Body)
	checkError(err)

	os.Mkdir("/go/files/" + strconv.FormatInt(id, 10), 0777)
	checkError(err)

	file, err := os.OpenFile("/go/files/" + strconv.FormatInt(id, 10) + "/" + strconv.FormatInt(imageId, 10) + ".png", os.O_CREATE | os.O_RDWR, 0666)
	checkError(err)
	defer file.Close()

	file.Write(imageBody)
}

func createImageLink(image string, u *url.URL) string {
	imageUrl, err := url.Parse(image)
	if err == nil {
		return checkUrl(imageUrl, u)
	}
	return "error"
}

func checkUrl(u *url.URL, base *url.URL) string {
	if u.Scheme == "" {
		u.Scheme = "http"
	}

	if u.Host == "" {
		u.Host = base.Host
	}

	if u.Path[0] != "/"[0] {
		// TODO: fix relative image path
	}

	return u.String()
}
