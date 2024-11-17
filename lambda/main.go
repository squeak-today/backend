package main

// https://github.com/cohere-ai/cohere-go
// https://docs.cohere.com/v2/docs/cohere-works-everywhere#cohere-platform
// GOOGLE_API_KEY = "api-key" COHERE_API_KEY="api-key" STORY_BUCKET_NAME="story-generation-bucket-dev" go run .

// compile to binary
// GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap .
// MUST BE NAMED BOOSTRAP FOR THE NEW provided.al2 RUNTIME
// zip function.zip bootstrap

import (
    "os"
    "context"
    "log"
    "github.com/aws/aws-lambda-go/lambda"

    "time"
    "strings"
)

// on current lambda specs
// -> 1 language, 2 cefr, 4 subjects took ~120 seconds (B1, B2)
func handler(ctx context.Context) error {
    log.Println("Executing Aya Story Generation...")

    languages := []string{"French"}
	language_ids := map[string]string{
		"French": "fr",
	}
    cefrLevels := []string{"A1", "B2", "C2"} // keep minimal for testing (currently lambda times out)
	
    // subjects := []string{"World", "Investing", "Politics", "Sports", "Arts"}
	subjects := []string{"Politics"}

    // generate web results
	webResults := make(map[string]string)
	webSources := make(map[string][]Result)
	for i := 0; i < len(subjects); i++ {
		subject := subjects[i]
		resp, _ := webSearch("today " + subject + " news", 20)
		webSources[subject] = resp.Results
		webResults[subject] = buildInfoBlockFromTavilyResponse(resp)
	}

    // o(a*6*b*3), so maybe considering increasing lambda timeout
    for i := 0; i < len(languages); i++ {
        for j := 0; j < len(cefrLevels); j++ {
			for k := 0; k < len(subjects); k++ {
				
				// Story Generation
				storyResponse, err := generateStory(languages[i], cefrLevels[j], subjects[k])
    
				if err == nil {
					story := storyResponse.Message.Content[0].Text
					log.Println("Story:", story)

					words, sentences := getWordsAndSentences(story)
					dictionary, _ := generateTranslations(words, sentences, language_ids[languages[i]])
					body, _ := buildStoryBody(story, dictionary)

					current_time := time.Now().UTC().Format("2006-01-02")

					path := strings.ToLower(languages[i]) + "/" + cefrLevels[j] + "/" + subjects[k] + "/" + "Story/"
					push_path := path + cefrLevels[j] + "_Story_" + subjects[k] + "_" + current_time + ".json"
			
					if err := uploadStoryS3(
						os.Getenv("STORY_BUCKET_NAME"),
						push_path, body,
					); err != nil {
						log.Println(err)
						return err
					}
				} else {
					log.Println(err)
					return err
				}

				// News Generation
				newsResp, err := generateNewsArticle(languages[i], cefrLevels[j], "today " + subjects[k] + " news", webResults[subjects[k]])

				if err == nil {
					words, sentences := getWordsAndSentences(newsResp.Text)
					dictionary, _ := generateTranslations(words, sentences, language_ids[languages[i]])
					body, _ := buildNewsBody(newsResp.Text, dictionary, webSources[subjects[k]])

					current_time := time.Now().UTC().Format("2006-01-02")

					path := strings.ToLower(languages[i]) + "/" + cefrLevels[j] + "/" + subjects[k] + "/" + "News/"
					push_path := path + cefrLevels[j] + "_News_" + subjects[k] + "_" + current_time + ".json"

					if err := uploadStoryS3(
						os.Getenv("STORY_BUCKET_NAME"),
						push_path, body,
					); err != nil {
						log.Println(err)
						return err
					}
				} else {
					log.Println(err)
					return err
				}
			}
        }
    }

    return nil
}

func main() {
    lambda.Start(handler)
}