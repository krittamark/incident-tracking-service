package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

type IncidentCreateRequestBody struct {
	Type                     string `json:"incident_type"`
	Description              string `json:"incident_description"`
	ExactLocation            string `json:"exact_location"`
	ExactLocationDescription string `json:"exact_location_description"`
	ImpactLevel              int    `json:"impact_level"`
	Priority                 string `json:"priority"`
	ReportedBy               string `json:"reported_by"`
}

var dynamoClient *dynamodb.Client

func init() {

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load AWS configuration: %v", err)
	}

	dynamoClient = dynamodb.NewFromConfig(cfg)
	fmt.Println("This is the init function for the incidenttrackingservice_create package.")

	dynamodbTableName := os.Getenv("DYNAMODB_TABLE_NAME")
	if dynamodbTableName == "" {
		log.Fatal("DYNAMODB_TABLE_NAME environment variable is not set")
	}
	fmt.Printf("DYNAMODB_TABLE_NAME: %s\n", dynamodbTableName)

}

func handleRequest(ctx context.Context, event json.RawMessage) error {
	fmt.Println("Received event:", string(event))

	var requestBody IncidentCreateRequestBody
	err := json.Unmarshal(event, &requestBody)
	if err != nil {
		log.Printf("Error unmarshalling request body: %v", err)
		return fmt.Errorf("invalid request body: %w", err)
	}

	fmt.Printf("Parsed request body: %+v\n", requestBody)

	if err != nil {
		log.Printf("Error loading AWS configuration: %v", err)
		return fmt.Errorf("unable to load AWS configuration: %w", err)
	}

	// Prepare the item to be inserted into DynamoDB
	item := map[string]types.AttributeValue{
		"incident_id":                &types.AttributeValueMemberS{Value: uuid.NewString()},
		"incident_type":              &types.AttributeValueMemberS{Value: requestBody.Type},
		"incident_description":       &types.AttributeValueMemberS{Value: requestBody.Description},
		"exact_location":             &types.AttributeValueMemberS{Value: requestBody.ExactLocation},
		"exact_location_description": &types.AttributeValueMemberS{Value: requestBody.ExactLocationDescription},
		"impact_level":               &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", requestBody.ImpactLevel)},
		"priority":                   &types.AttributeValueMemberS{Value: requestBody.Priority},
		"reported_by":                &types.AttributeValueMemberS{Value: requestBody.ReportedBy},
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(os.Getenv("DYNAMODB_TABLE_NAME")),
		Item:      item,
	}
	_, err = dynamoClient.PutItem(ctx, input)
	if err != nil {
		log.Printf("Error putting item in DynamoDB: %v", err)
		return fmt.Errorf("unable to put item in DynamoDB: %w", err)
	}
	return nil
}

func main() {
	lambda.Start(handleRequest)
}
