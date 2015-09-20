package main

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"io/ioutil"
	"net/url"
	"os"
	"strconv"
	"path"
	"image"
	_ "image/gif"
	_ "image/png"
	_ "image/jpeg"
)

var connectionString = os.Getenv("connectionString")
var db, dbError = sql.Open("mysql", "root:@tcp(localhost:3306)/gowc?charset=utf8")

const statusProcessing = 0
const statusDone = 1
const statusErrorLoad = 2
const statusErrorParse = 3

const imageStatusProcessing = 0
const imageStatusDone = 1
const imageStatusErrorLink = 2
const imageStatusErrorLoad = 3
const imageStatusErrorSave = 4
const imageStatusErrorSize = 5
const imageStatusErrorDimension = 6

type Error struct {
	Msg  string
	Err error
}
func (e *Error) Error() string { return e.Msg + ": " + e.Err.Error() }

type Job struct {
	Id int `json:"id"`
	Url string `json:"url"`
	Status string `json:"status"`
	Images []JobImage `json:"images"`
}

type JobImage struct {
	Url string `json:"url"`
	Link string `json:"link"`
	Download string `json:"download"`
	Status string `json:"status"`
	Type string `json:"type"`
	Size int `json:"size"`
	Height int `json:"height"`
	Width int `json:"width"`
}

type Request struct {
	Link string `json:"link"`
}

func main() {

	db.Ping()

	checkError(dbError)

	defer db.Close()

	r := gin.Default()
	r.GET("/", index)
	r.POST("/parsers", parse)
	r.GET("/parsers/:id", getJob)
	r.NoRoute(func (c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
	})
	r.Any("/api.json", func (c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "origin, content-type, accept")
		c.File("./api.json")
	})
	r.Run(":8080")
}

func getJob(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "origin, content-type, accept")
	id := c.Param("id")
	json, err := selectJob(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "error", "error": err})
		return
	}

	c.JSON(http.StatusOK, json)
}

func selectJob(id string) (data Job, err error) {

	var uid int
	var url string
	var status int

	err = db.QueryRow("SELECT id, url, status FROM url WHERE id = ?", id).Scan(&uid, &url, &status)
	if err != nil {
		return Job{}, &Error{Msg:"job not found", Err: err}
	}

	data.Id = uid
	data.Url = url
	data.Status = linkStatus(status)

	data, err = selectImages(id, data)
	if err != nil {
		return Job{}, err
	}

	return data, nil
}

func selectImages(id string, job Job) (Job, error) {

	rows, err := db.Query("SELECT id, url, link, status, type, size, height, width FROM image WHERE url_id = ?", id)
	defer rows.Close()
	if err != nil {
		return Job{}, err
	}

	for rows.Next() {
		var uid int
		var url string
		var link string
		var status int
		var contentType string
		var size int
		var height int
		var width int
		err = rows.Scan(&uid, &url, &link, &status, &contentType, &size, &height, &width)
		if err != nil {
			return Job{}, err
		}

		var temp JobImage

		temp.Url = url
		temp.Link = link
		temp.Download = "http://localhost:8080/files/" + id + "/" + strconv.Itoa(uid) + ".jpg"
		temp.Status = imageStatus(status)
		temp.Size = size
		temp.Height = height
		temp.Width = width
		temp.Type = contentType

		job.Images = append(job.Images, temp)
	}

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
	case imageStatusErrorSize:
		return "error size"
	case imageStatusErrorDimension:
		return "error dimension"
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
	c.JSON(http.StatusOK, gin.H{"use": []string{"http://localhost:8080/api"}, "message": "Hey! You can use such links"})
}

func parse(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "origin, content-type, accept")

	var json Request
	err := c.BindJSON(&json)
	if err != nil || json.Link == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Incrorrect request"})
		return
	}

	link := json.Link

	u, err := url.Parse(link)
	if err != nil {
		c.JSON(400, gin.H{"message": "Broken link", "link": link})
		return
	}

	id := insertUrl(u.String())

	go grab(u, id)

	c.JSON(http.StatusCreated, gin.H{"message": "processing", "id": id, "link": link})
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
	stmt, err := db.Prepare("INSERT INTO image(link, url_id, url, status, type, size, height, width) values(?, ?, ?, ?, ?, ?, ?, ?)")
	checkError(err)
	res, err := stmt.Exec(url, urlId, "", imageStatusProcessing, "", 0, 0, 0)
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

func imageStatusSize (id int64) {
	imageUpdateStatus(id, imageStatusErrorSize)
}

func imageStatusDimension (id int64) {
	imageUpdateStatus(id, imageStatusErrorDimension)
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

func imageUpdateType(id int64, contentType string) {
	stmt, err := db.Prepare("UPDATE image SET type = ? WHERE id = ?")
	checkError(err)
	res, err := stmt.Exec(contentType, id)
	checkError(err)
	id, err = res.LastInsertId()
	checkError(err)
}

func imageUpdateSize(id int64, size int64) {
	stmt, err := db.Prepare("UPDATE image SET size = ? WHERE id = ?")
	checkError(err)
	res, err := stmt.Exec(size, id)
	checkError(err)
	id, err = res.LastInsertId()
	checkError(err)
}

func imageUpdateDimension(id int64, width int, height int) {
	stmt, err := db.Prepare("UPDATE image SET width = ?, height = ? WHERE id = ?")
	checkError(err)
	res, err := stmt.Exec(width, height, id)
	checkError(err)
	id, err = res.LastInsertId()
	checkError(err)
}

func downloadImage(imageUrl string, u *url.URL, id int64) error {

	imageId := insertImageUrl(imageUrl, id)

	imageLink, err := createImageLink(imageUrl, u)
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

	contentType := resp.Header.Get("Content-Type")
	imageUpdateType(imageId, contentType)

	ext := path.Ext(imageLink)

	imageBody, err := ioutil.ReadAll(resp.Body)
	checkError(err)

	os.Mkdir("./files/", 0777)
	os.Mkdir("./files/" + strconv.FormatInt(id, 10), 0777)

	fileName := "./files/" + strconv.FormatInt(id, 10) + "/" + strconv.FormatInt(imageId, 10) + ext

	file, err := os.OpenFile(fileName, os.O_CREATE | os.O_RDWR, 0666)
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

	info, err := os.Lstat(fileName)
	if err != nil {
		imageStatusSize(imageId)
		return err
	}

	imageUpdateSize(imageId, info.Size())


	ifile, err := os.Open(fileName)
	defer ifile.Close();
	if err != nil {
		imageStatusDimension(imageId)
		return err
	}
	imageConfig, _, err := image.DecodeConfig(ifile)
	if err != nil {
		imageStatusDimension(imageId)
		return err
	}

	imageUpdateDimension(imageId, imageConfig.Width, imageConfig.Height)

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
