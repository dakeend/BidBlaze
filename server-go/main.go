package main

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Auction struct {
	ID           int    `json:"id"`
	ProductName string `json:"productName"`
	CurrentPrice int   `json:"currentPrice"`
	Status       string `json:"status"`
}

var currentAuction = Auction{
	ID:           1,
	ProductName:  "天然翡翠吊坠",
	CurrentPrice: 900,
	Status:       "running",
}

func main() {
	r := gin.Default()

	r.Use(cors.Default())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Go server is running",
		})
	})

	r.GET("/api/auctions/current", func(c *gin.Context) {
		c.JSON(http.StatusOK, currentAuction)
	})

	r.POST("/api/auctions/:id/bid", func(c *gin.Context) {
		currentAuction.CurrentPrice += 50

		c.JSON(http.StatusOK, gin.H{
			"message":      "出价成功",
			"currentPrice": currentAuction.CurrentPrice,
		})
	})

	r.Run(":8080")
}