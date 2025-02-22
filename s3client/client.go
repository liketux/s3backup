package s3client

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"s3backup/log"
	"os"
)

// CreateS3Client creates an S3 client using environment variables if present; else AWS creds file
// 2. Use the specified credential file
func CreateS3Client(credFile string, profile string, region string, endpoint string) (*s3.S3, error) {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

	session := session.Must(session.NewSession())

	var creds *credentials.Credentials

	if accessKey == "" && secretAccessKey == "" {
		// Missing both of the required environment variables
		log.Info.Println("Environment variables missing to create client: 'AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY'")
	} else if accessKey == "" {
		log.Info.Println("Environment variable missing: 'AWS_ACCESS_KEY_ID'")
	} else if secretAccessKey == "" {
		log.Info.Println("Environment variable missing: 'AWS_SECRET_ACCESS_KEY'")

	} else {
		log.Info.Println("Loaded AWS credentials from environment variables")
		creds = credentials.NewEnvCredentials()
	}

	if creds == nil {
		log.Info.Printf("Attempting to create S3 client with specified credential file and profile: [%s | %s]\n", credFile, profile)
		creds = credentials.NewSharedCredentials(credFile, profile)
	}

	if creds == nil {
		return nil, errors.New("failed to retrieve S3 client access key id and access key secret")
	}

	return s3.New(session, &aws.Config{Region: aws.String(region), Credentials: creds, Endpoint: aws.String(endpoint)}), nil
}
