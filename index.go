package main

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"fmt"
	"io/ioutil"
)

func main() {
	//url := "http://apostrophe.com.ua/"

	r := gin.Default()
	r.GET("/", index)
	r.GET("/parse", parse)
	r.Run(":8080")
}

func index(c *gin.Context) {
	c.JSON(200, gin.H{"message": "Hey"})
}

func parse(c *gin.Context) {
	url := "http://apostrophe.com.ua/"
	resp, err := http.Get(url)
	if err != nil {
		c.JSON(400, gin.H{"message": err, "restonse": resp})
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	fmt.Printf("\n%v\n\n", string(body))
	//c.JSON(200, gin.H{"message": "OK", "restonse": resp})
}
