package main

import (
	"context"
	"log"
	"encoding/json"
	"io"
	"os"
	"bytes"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/gin"
	"github.com/gin-gonic/gin"
	"net/http"
)

type TranslateResponse struct {
	Data struct {
		Translations []struct {
			TranslatedText string `json:"translatedText"`
		} `json:"translations"`
	} `json:"data"`
}

// STORY_BUCKET_NAME="story-generation-bucket-dev" go run .
// GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap .
// zip front-function.zip bootstrap

var ginLambda *ginadapter.GinLambda

func init() {
	log.Println("Gin cold start")
	router := gin.Default()

	AllowOrigin := "*"


	router.Use(func(c *gin.Context) {
		// * accepts all origins, change for production
		c.Writer.Header().Set("Access-Control-Allow-Origin", AllowOrigin)
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		c.Writer.Header().Set("Access-Control-Max-Age", "3600")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	})


	router.GET("/story", func(c *gin.Context) {
		language := c.Query("language")
		cefr := c.Query("cefr")
		subject := c.Query("subject")
		

		if language == "" || cefr == "" || subject == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "language, cefr, and subject parameter is required!",
			})
			return
		}

		// theres no check for valid language or cefr yet

		content, err := pullContent(language, cefr, subject, "Story")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "content retrieval failed!",
			})
			return
		}

		c.JSON(http.StatusOK, content.ToMap())
	})

	router.GET("/news", func(c *gin.Context) {
		language := c.Query("language")
		cefr := c.Query("cefr")
		subject := c.Query("subject")

		if language == "" || cefr == "" || subject == ""{
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "language, cefr, and subject parameter is required!",
			})
			return
		}

		// theres no check for valid language or cefr yet

		content, err := pullContent(language, cefr, subject, "News")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "content retrieval failed!",
			})
			return
		}

		c.JSON(http.StatusOK, content.ToMap())
	})

	router.POST("/translate", func(c *gin.Context) {
		var infoBody struct {
			Sentence string `json:"sentence"`
			Source string `json:"source"`
			Target string `json:"target"`
		}

		if err := c.ShouldBindJSON(&infoBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		googleAPIKey := os.Getenv("GOOGLE_API_KEY")
		query := []string{infoBody.Sentence}
		translatePayload := map[string]interface{}{
			"q": query,
			"source": infoBody.Source,
			"target": infoBody.Target,
			"format": "text",
		}

		jsonData, err := json.Marshal(translatePayload)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Translate payload marshalling failed!",
			})
			return
		}

		req, err := http.NewRequest("POST", "https://translation.googleapis.com/language/translate/v2?key=" + googleAPIKey, bytes.NewBuffer(jsonData))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Request to GCP failed!",
			})
			return
		}

		req.Header.Set("Content-Type", "application/json")


		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Request to GCP failed!",
			})
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Request to GCP failed!",
			})
			return
		}

		var result TranslateResponse
		if err := json.Unmarshal(body, &result); err != nil {
			log.Println(string(body))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "TranslateResponse unmarshalling failed!",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"sentence": result.Data.Translations[0].TranslatedText,
		})
	})

	ginLambda = ginadapter.New(router)
}

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return ginLambda.ProxyWithContext(ctx, req)
}

func main() {
	lambda.Start(Handler)
}
