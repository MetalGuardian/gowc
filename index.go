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
	"path"
)

var db, dbError = sql.Open("sqlite3", "./database.db")

const statusProcessing = 0
const statusDone = 1
const statusErrorLoad = 2
const statusErrorParse = 3

const imageStatusProcessing = 0
const imageStatusDone = 1
const imageStatusErrorLink = 2
const imageStatusErrorLoad = 3
const imageStatusErrorSave = 4

type Error struct {
	Msg  string
	Err error
}
func (e *Error) Error() string { return e.Msg + ": " + e.Err.Error() }

type Job struct {
	Id string `json:"id"`
	Url string `json:"url"`
	Status string `json:"status"`
	Images []JobImage `json:"images"`
}

type JobImage struct {
	Id string `json:"id"`
	Url string `json:"url"`
	Link string `json:"link"`
	Status string `json:"status"`
	Type string `json:"type"`
	Size int `json:"size"`
	Height int `json:"height"`
	Width int `json:"width"`
}

func main() {

	db.Ping()

	checkError(dbError)

	defer db.Close()

	r := gin.Default()
	r.GET("/", index)
	r.GET("/parse", parse)
	r.GET("/parse/:id", getJob)
	r.Run(":8080")
}

func getJob(c *gin.Context) {
	id := c.Param("id")
	json, err := selectJob(id)
	if err != nil {
		c.JSON(400, gin.H{"message": "error", "error": err})
		return
	}

	c.JSON(200, json)
}

func selectJob(id string) (data Job, err error) {
	data = Job{Id: id, Url: ""}

	rows, err := db.Query("SELECT id, url, status FROM url WHERE id = ?", id)
	if err != nil {
		return Job{}, err
	}

	for rows.Next() {
		var uid int
		var url string
		var status int
		err = rows.Scan(&uid, &url, &status)
		if err != nil {
			return Job{}, err
		}
		data.Url = url
		data.Status = linkStatus(status)

		data, err = selectImages(id, data)
		if err != nil {
			return Job{}, err
		}
	}

	return data, nil
}

func selectImages(id string, job Job) (Job, error) {

	rows, err := db.Query("SELECT id, url, link, status, type, size, height, width FROM image WHERE url_id = ?", id)
	if err != nil {
		fmt.Println(err)
		return Job{}, err
	}

	for rows.Next() {
		var uid int
		var url string
		var link string
		var status int
		var typeField string
		var size int
		var height int
		var width int
		err = rows.Scan(&uid, &url, &link, &status, &typeField, &size, &height, &width)
		if err != nil {
			fmt.Println(err)
			return Job{}, err
		}

		var temp JobImage

		temp.Url = url
		temp.Status = imageStatus(status)
		temp.Size = 0
		temp.Height = 0
		temp.Width = 0
		temp.Type = "testing"

		job.Images = append(job.Images, temp)
	}

	fmt.Println(job)

	return job, nil
}

func linkStatus(status int) string {
	switch status {
	case statusProcessing:
		return "processing"
	case statusErrorParse:
		return "error parsing html"
	case statusErrorLoad:
		return "error loading link"
	case statusDone:
		return "done"
	}

	return "Unknown status"
}

func imageStatus(status int) string {
	switch status {
	case imageStatusProcessing:
		return "processing"
	case imageStatusErrorLink:
		return "error link"
	case imageStatusErrorLoad:
		return "error loading"
	case imageStatusErrorSave:
		return "error saving"
	case imageStatusDone:
		return "done"
	}

	return "Unknown status"
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func index(c *gin.Context) {
	c.JSON(200, gin.H{"message": "Hey"})
}

func parse(c *gin.Context) {
	link := "http://fie.org/fie/structure/manufacturers"
	//link = "http://www.zaryachem.com/ru"
	//link = "http://vintage.com.ua"

	u, err := url.Parse(link)
	if err != nil {
		c.JSON(400, gin.H{"message": "Broken link", "link": link})
		return
	}

	id := insertUrl(u.String())

	go grab(u, id)

	c.JSON(200, gin.H{"message": "processing", "jobId": id})
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
	stmt, err := db.Prepare("INSERT INTO image(link, url_id, url, status, link, type, size, height, width) values(?, ?, ?, ?, ?, ?, ?, ?, ?)")
	checkError(err)
	res, err := stmt.Exec(url, urlId, "", imageStatusProcessing, "", "", 0, 0, 0)
	checkError(err)
	id, err := res.LastInsertId()
	checkError(err)

	return id
}

func grab(u *url.URL, id int64) error {
	resp, err := http.Get(u.String())
	if err != nil {
		linkErrorLoad(id)
		return err
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		linkErrorParse(id)
		return err
	}

	getImages(doc, u, id)

	complete(id)

	fmt.Println(id)

	return nil
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

func complete(id int64) {
	linkUpdateStatus(id, statusDone)
}

func linkErrorLoad (id int64) {
	linkUpdateStatus(id, statusErrorLoad)
}

func linkErrorParse (id int64) {
	linkUpdateStatus(id, statusErrorParse)
}

func linkUpdateStatus(id int64, status int) {
	stmt, err := db.Prepare("UPDATE url SET status = ? WHERE id = ?")
	checkError(err)
	res, err := stmt.Exec(status, id)
	checkError(err)
	id, err = res.LastInsertId()
	checkError(err)
}

func imageStatusLink (id int64) {
	imageUpdateStatus(id, imageStatusErrorLink)
}

func imageStatusLoad (id int64) {
	imageUpdateStatus(id, imageStatusErrorLoad)
}

func imageStatusSave (id int64) {
	imageUpdateStatus(id, imageStatusErrorSave)
}

func imageStatusSetDone (id int64) {
	imageUpdateStatus(id, imageStatusDone)
}

func imageUpdateStatus(id int64, status int) {
	stmt, err := db.Prepare("UPDATE image SET status = ? WHERE id = ?")
	checkError(err)
	res, err := stmt.Exec(status, id)
	checkError(err)
	id, err = res.LastInsertId()
	checkError(err)
}

func imageUpdateUrl(id int64, url string) {
	stmt, err := db.Prepare("UPDATE image SET url = ? WHERE id = ?")
	checkError(err)
	res, err := stmt.Exec(url, id)
	checkError(err)
	id, err = res.LastInsertId()
	checkError(err)
}

func downloadImage(image string, u *url.URL, id int64) error {

	imageId := insertImageUrl(image, id)

	imageLink, err := createImageLink(image, u)
	if err != nil {
		imageStatusLink(imageId)
		return err
	}
	imageUpdateUrl(imageId, imageLink)

	resp, err := http.Get(imageLink)
	if err != nil {
		imageStatusLoad(imageId)
		return err
	}

	ext := path.Ext(imageLink)

	imageBody, err := ioutil.ReadAll(resp.Body)
	checkError(err)

	os.Mkdir("./files/", 0777)
	os.Mkdir("./files/" + strconv.FormatInt(id, 10), 0777)

	file, err := os.OpenFile("./files/" + strconv.FormatInt(id, 10) + "/" + strconv.FormatInt(imageId, 10) + ext, os.O_CREATE | os.O_RDWR, 0666)
	defer file.Close()
	if err != nil {
		imageStatusSave(imageId)
		return err
	}

	_, err = file.Write(imageBody)
	if err != nil {
		imageStatusSave(imageId)
		return err
	}

	imageStatusSetDone(imageId)

	return nil
}

func createImageLink(image string, u *url.URL) (imgUrl string, err error) {
	imageUrl, err := url.Parse(image)
	if err == nil {
		return checkUrl(imageUrl, u), nil
	}
	return "", &Error{"parse", err}
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
