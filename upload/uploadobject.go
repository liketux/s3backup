package upload

import "time"

// UploadObject represents an object to be uploaded to S3
type UploadObject struct {
	PathToFile string
	S3FileName string
	Bucket     string
	BucketDir  string
	Endpoint   string
	Manipulate bool
	Timeout    time.Duration
	NumWorkers int
	PartSize   int
}
