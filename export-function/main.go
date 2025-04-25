// Handle events from DynamoDB stream and export sample metrics to Dynatrace
package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

var dtEnv string
var dtToken string

func init() {
	// Grab the Dynatrace environment name from the environment variable
	dtEnv = os.Getenv("DynatraceEnv")
	if dtEnv == "" {
		log.Fatal("missing environment variable DynatraceEnv")
	}
	cfg, _ := config.LoadDefaultConfig(context.Background())

	// Create Secrets Manager client
	svc := secretsmanager.NewFromConfig(cfg)

	// Retrieve the secret value for the Dynatrace API token from AWS Secrets Manager
	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String("arn:aws:secretsmanager:eu-west-2:550844279723:secret:dynatrace-nonprod-yAabha"),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}

	result, err := svc.GetSecretValue(context.TODO(), input)
	if err != nil {
		log.Fatal(err.Error())
	}

	dtToken = *result.SecretString
}

func main() {
	lambda.Start(handler)
}

func handler(ctx context.Context, e events.DynamoDBEvent) {

	// Iterate over each record in the DynamoDB stream event
	for _, r := range e.Records {
		log.Println("New record -", r.Change.NewImage)

		// Grab the city and state attributes from the new user record
		var city string
		var state string
		for k, v := range r.Change.NewImage {
			//log.Println("arrtibute info", k, v, v.DataType())
			if k == "city" {
				city = v.String()
			}
			if k == "state" {
				state = v.String()
			}
		}

		// Send the a count delta of 1 for the city and state from the request to Dynatrace
		log.Println("City: ", city)
		log.Println("State: ", state)
		body := strings.NewReader(`new_user_count,city="` + city + `",state="` + state + `" count,delta=1`)
		req, err := http.NewRequest(http.MethodPost, "https://"+dtEnv+".live.dynatrace.com/api/v2/metrics/ingest", body)
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Set("Authorization", "Api-Token "+dtToken)
		req.Header.Set("Content-Type", "text/plain")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		// Check the response status code
		// 202 means the request was accepted
		// Print the response body for debugging
		if resp.StatusCode == 202 {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
			}
			bodyString := string(bodyBytes)
			log.Println(bodyString)
		} else {
			log.Println("Error: ", resp.StatusCode)
		}
	}
}
