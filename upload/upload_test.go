package upload

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/s3"
	"s3backup/log"
	"s3backup/rpolicy"
	"s3backup/s3client"
	"s3backup/util"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Test variables
var svc *s3.S3
var bucket string
var s3FileName string
var policy rpolicy.RotationPolicy
var timeout time.Duration

var dailyRetentionCount int
var dailyRetentionPeriod time.Duration

var weeklyRetentionCount int
var weeklyRetentionPeriod time.Duration

var testFileName string
var pathToTestFile string
var testUploadObjectManipulated UploadObject
var testUploadObjectNotManipulated UploadObject

var bigS3FileName string
var pathToBigFile string
var bigFileSize int64
var bigTestUploadObject UploadObject

var awsForbiddenBucket string

// Setup testing
func init() {
	log.Init(ioutil.Discard, ioutil.Discard, ioutil.Discard)

	awsCredentials := os.Getenv("AWS_CRED_FILE")
	awsProfile := os.Getenv("AWS_PROFILE")
	awsRegion := os.Getenv("AWS_REGION")
	awsBucket := os.Getenv("AWS_BUCKET_UPLOAD")
	awsForbiddenBucket = os.Getenv("AWS_BUCKET_FORBIDDEN")

	s3svc, err := s3client.CreateS3Client(awsCredentials, awsProfile, awsRegion)
	if err != nil {
		log.Error.Println(err)
		os.Exit(1)
	}

	svc = s3svc

	bucket = awsBucket

	dailyRetentionCount = 6
	dailyRetentionPeriod = 140

	weeklyRetentionCount = 4
	weeklyRetentionPeriod = 280

	s3FileName = "test_file"
	timeout = time.Second * 3600

	testFileName = "testBackupFile"
	pathToTestFile = "../" + testFileName

	testUploadObjectManipulated = UploadObject{
		PathToFile: pathToTestFile,
		S3FileName: s3FileName,
		BucketDir:  "",
		Bucket:     bucket,
		Timeout:    timeout,
		NumWorkers: 5,
		PartSize:   50,
		Manipulate: true,
	}

	testUploadObjectNotManipulated = UploadObject{
		PathToFile: pathToTestFile,
		S3FileName: s3FileName,
		BucketDir:  "",
		Bucket:     bucket,
		Timeout:    timeout,
		NumWorkers: 5,
		PartSize:   50,
		Manipulate: false,
	}

	err = util.CreateFile(pathToTestFile, []byte("this is just a little test file"))
	if err != nil {
		log.Error.Println("failed to create file required for testing")
	}

	bigFileSize = int64(1000 * 1024 * 1024) // 1GiB
	bigS3FileName = "bigS3File"
	pathToBigFile = "../" + bigS3FileName

	bigTestUploadObject = UploadObject{
		PathToFile: pathToBigFile,
		S3FileName: bigS3FileName,
		BucketDir:  "",
		Bucket:     bucket,
		Timeout:    timeout,
		NumWorkers: 5,
		PartSize:   50,
		Manipulate: true,
	}

	err = util.CreateBigFile(pathToBigFile, bigFileSize)
	if err != nil {
		log.Error.Println("failed to create file required for testing")
	}

	// Not critical to run this up but can get costly if no lifecycle policy in place to clean up dead multiparts
	util.CleanUpMultiPartUploads(svc, bucket)

	policy = rpolicy.RotationPolicy{
		DailyRetentionPeriod:   time.Second * dailyRetentionPeriod,
		DailyRetentionCount:    dailyRetentionCount,
		DailyPrefix:            "daily_",
		WeeklyRetentionPeriod:  time.Second * weeklyRetentionPeriod,
		WeeklyRetentionCount:   weeklyRetentionCount,
		WeeklyPrefix:           "weekly_",
		MonthlyPrefix:          "monthly_",
		EnforceRetentionPeriod: false,
	}
}

//----------------------------------------------
//
//                Upload Tests
//
//----------------------------------------------

//----------------------------------------------
//
// Positive Upload Testing
//	1: Upload a single file
//	2: Upload a single file with justUploadIt set to true
//	3: Upload 50 files
//	4: Upload a Significantly Large File (250MiB)
//	5: Attempt to upload a file with dry run set to true
//	6: Upload file with bucket dir specified
//
//----------------------------------------------

// Test 1 - Positive Upload Testing
//	Upload a Single File
func TestUploadSingleFile(t *testing.T) {
	err := util.EmptyBucket(svc, bucket)
	if err != nil {
		t.Error("failed to empty bucket")
	}

	prefix := util.GetKeyType(policy, time.Now())
	s3FileName, err := UploadFile(svc, testUploadObjectManipulated, prefix, false)
	if err != nil {
		t.Error(fmt.Sprintf("expected to upload single file without any error: %v", err))
	}

	bucketContents, err := s3client.GetBucketContents(svc, bucket)
	if err != nil {
		t.Error("failed to retrieve bucket contents")
	}

	if !util.CheckBucketSize(bucketContents, 1) {
		t.Error("expected bucket size to be 1")
	}

	if !util.FindKeyInBucket(s3FileName, bucketContents) {
		t.Error("exepected to find key in bucket: " + s3FileName)
	}

}

// Test 2 - Positive Upload Testing
//	Upload a single file with manipulation set to false
func TestJustUploadIt(t *testing.T) {
	err := util.EmptyBucket(svc, bucket)
	if err != nil {
		t.Error("failed to empty bucket")
	}

	prefix := util.GetKeyType(policy, time.Now())
	_, err = UploadFile(svc, testUploadObjectNotManipulated, prefix, false) // Set justUploadIt to true
	if err != nil {
		t.Error("expected to upload single file without any error")
	}

	bucketContents, err := s3client.GetBucketContents(svc, bucket)
	if err != nil {
		t.Error("failed to retrieve bucket contents")
	}

	if !util.CheckBucketSize(bucketContents, 1) {
		t.Error("expected bucket size to be 1")
	}

	if !util.FindKeyInBucket(s3FileName, bucketContents) { // Notice no modification to file name for just upload it
		t.Error("exepected to find key in bucket: " + s3FileName)
	}
}

// Test 3 - Positive Upload Testing
//	Upload 50 Files
func TestUpload50Files(t *testing.T) {
	err := util.EmptyBucket(svc, bucket)
	if err != nil {
		t.Error("failed to empty bucket")
	}

	testUploadMultipleObject := UploadObject{
		PathToFile: pathToTestFile,
		S3FileName: s3FileName,
		BucketDir:  "",
		Bucket:     bucket,
		Timeout:    timeout,
		NumWorkers: 5,
		PartSize:   50,
		Manipulate: true,
	}

	bucketKeys := []string{}

	for i := 0; i < 50; i++ {
		testUploadMultipleObject.S3FileName = s3FileName + strconv.Itoa(i)

		prefix := util.GetKeyType(policy, time.Now())
		bucketKey, err := UploadFile(svc, testUploadMultipleObject, prefix, false)
		if err != nil {
			t.Error(fmt.Sprintf("expected to successfully upload file '%d' in bulk upload of 50 files", i))
		}
		bucketKeys = append(bucketKeys, bucketKey)
		time.Sleep(1) // Ensure there is a small delay between uploading files
	}

	bucketContents, err := s3client.GetBucketContents(svc, bucket)
	if err != nil {
		t.Error("failed to retrieve bucket contents")
	}

	if !util.CheckBucketSize(bucketContents, 50) {
		t.Error("expected bucket size to be 50")
	}

	for _, bucketKey := range bucketKeys {
		if !util.FindKeyInBucket(bucketKey, bucketContents) {
			t.Error("expected to find key in bucket: " + bucketKey)
		}
	}
}

// Test 4 - Positive Upload Testing
//	Upload a Significantly Large File (250MiB)
func TestUpload250MBFile(t *testing.T) {
	err := util.EmptyBucket(svc, bucket)
	if err != nil {
		t.Error("failed to empty bucket")
	}

	prefix := util.GetKeyType(policy, time.Now())
	bucketKey, err := UploadFile(svc, bigTestUploadObject, prefix, false)
	if err != nil {
		t.Error(fmt.Sprintf("failed to upload big file of size: %v bytes", bigFileSize))
	}

	bucketContents, err := s3client.GetBucketContents(svc, bucket)
	if err != nil {
		t.Error("failed to retrieve bucket contents")
	}

	if !util.CheckBucketSize(bucketContents, 1) {
		t.Error("expected bucket size to be 1")
	}

	if !util.FindKeyInBucket(bucketKey, bucketContents) {
		t.Error("expected to find key in bucket: " + bucketKey)
	}
}

// Test 5 - Positive Upload Testing
//	Upload a file with dry run set to run
func TestUploadWithDryRun(t *testing.T) {
	err := util.EmptyBucket(svc, bucket)
	if err != nil {
		t.Error("failed to empty bucket")
	}

	prefix := util.GetKeyType(policy, time.Now())
	_, err = UploadFile(svc, testUploadObjectManipulated, prefix, true)
	if err != nil {
		t.Error(fmt.Sprintf("failed to upload big file of size: %v bytes", bigFileSize))
	}

	bucketContents, err := s3client.GetBucketContents(svc, bucket)
	if err != nil {
		t.Error("failed to retrieve bucket contents")
	}

	if !util.CheckBucketSize(bucketContents, 0) {
		t.Error("expected bucket size to be 0")
	}
}

// Test 6 - Positive Upload Testing
//	Upload a file with bucket dir specified
func TestUploadBucketDir(t *testing.T) {
	err := util.EmptyBucket(svc, bucket)
	if err != nil {
		t.Error("failed to empty bucket")
	}

	testUploadBucketDirObject := UploadObject{
		PathToFile: pathToTestFile,
		S3FileName: s3FileName,
		BucketDir:  "testdir/",
		Bucket:     bucket,
		Timeout:    timeout,
		NumWorkers: 5,
		PartSize:   50,
		Manipulate: true,
	}

	prefix := util.GetKeyType(policy, time.Now())
	bucketKey, err := UploadFile(svc, testUploadBucketDirObject, prefix, false)

	bucketContents, err := s3client.GetBucketContents(svc, bucket)
	if err != nil {
		t.Error("failed to retrieve bucket contents")
	}

	if !util.CheckBucketSize(bucketContents, 1) {
		t.Error("expected bucket size to be 1")
	}

	if !util.FindKeyInBucket(bucketKey, bucketContents) {
		t.Error("expected to find key in bucket: " + bucketKey)
	}
}

func TestJustUploadItWithBucket(t *testing.T) {

}

//----------------------------------------------
// Negative Testing
// 	1: Upload a file where the bucket has not been specified
//	2. Upload a file where the bucket has an invalid name
//	3: Upload a file that does not exist
//	4: Upload a file to a bucket without the appropriate permissions
//	5: Upload a file that exceeds the specified timeout period (60 seconds)
//
//----------------------------------------------

// Test 1 - Negative Upload Testing
//	Upload a file where the bucket has not been specified
func TestUploadInvalidBucketNotSpecified(t *testing.T) {
	expectedErrString := "invalid bucket specified, bucket must be specified"

	testUploadNoBucketObject := UploadObject{
		PathToFile: pathToTestFile,
		S3FileName: s3FileName,
		BucketDir:  "",
		Bucket:     "",
		Timeout:    timeout,
		NumWorkers: 5,
		PartSize:   50,
		Manipulate: true,
	}

	prefix := util.GetKeyType(policy, time.Now())
	_, err := UploadFile(svc, testUploadNoBucketObject, prefix, false)
	if err != nil && strings.Contains(err.Error(), expectedErrString) {
		// Pass
	} else {
		t.Error("expected upload to fail with system unable to find specified file, isntead got: " + err.Error())
	}
}

// Test 2 - Negative Upload Testing
//	Upload a file where the bucket has an invalid name
func TestUploadInvalidBucketBadName(t *testing.T) {
	expectedErrString := "status code: 400"

	testUploadInvalidBucketObject := UploadObject{
		PathToFile: pathToTestFile,
		S3FileName: s3FileName,
		BucketDir:  "",
		Bucket:     "badbucket*?",
		Timeout:    timeout,
		NumWorkers: 5,
		PartSize:   50,
		Manipulate: true,
	}

	prefix := util.GetKeyType(policy, time.Now())
	_, err := UploadFile(svc, testUploadInvalidBucketObject, prefix, false)
	if err != nil && strings.Contains(err.Error(), expectedErrString) {
		// Pass
	} else {
		t.Error("expected upload to fail with system unable to find specified file")
	}
}

// Test 3 - Negative Upload Testing
//	Upload a file that does not exist
func TestUploadInvalidFile(t *testing.T) {

	testUploadInvalidPathObject := UploadObject{
		PathToFile: "../this/should/../definitely/../../notexist",
		S3FileName: s3FileName,
		BucketDir:  "",
		Bucket:     bucket,
		Timeout:    timeout,
		NumWorkers: 5,
		PartSize:   50,
		Manipulate: true,
	}

	prefix := util.GetKeyType(policy, time.Now())
	_, err := UploadFile(svc, testUploadInvalidPathObject, prefix, false)
	if err != nil {
		// Pass
	} else {
		t.Error("expected upload to fail with system unable to find specified file")
	}
}

// Test 4 - Negative Upload Testing
//	Upload a file to a bucket without the appropriate permissions
func TestUploadForbiddenBucket(t *testing.T) {
	expectedErrString := "status code: 403"

	testUploadBadPermissionObject := UploadObject{
		PathToFile: pathToTestFile,
		S3FileName: s3FileName,
		BucketDir:  "",
		Bucket:     awsForbiddenBucket,
		Timeout:    timeout,
		NumWorkers: 5,
		PartSize:   50,
		Manipulate: true,
	}

	prefix := util.GetKeyType(policy, time.Now())
	_, err := UploadFile(svc, testUploadBadPermissionObject, prefix, false)
	if err != nil && strings.Contains(err.Error(), expectedErrString) {
		// Pass
	} else {
		t.Error("expected upload to fail with status code 403, instead got: " + err.Error())
	}
}

// Test 5 - Negative Upload Testing
//	Upload a file that exceeds the specified timeout period (60 seconds)
func TestUploadExceedTimeout(t *testing.T) {

	testUploadTimeoutObject := UploadObject{
		PathToFile: pathToBigFile,
		S3FileName: bigS3FileName,
		BucketDir:  "",
		Bucket:     bucket,
		Timeout:    time.Second * 10,
		NumWorkers: 5,
		PartSize:   50,
		Manipulate: true,
	}

	prefix := util.GetKeyType(policy, time.Now())
	_, err := UploadFile(svc, testUploadTimeoutObject, prefix, false)

	if err == nil && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Error(fmt.Sprintf("expected file upload to timeout. timeout specified was: %d seconds", timeout))
	}

}

// Test 6 - Negative Upload Testing
//	Upload a file with an invalid bucket directory
func TestUploadInvalidBucketDir(t *testing.T) {
	expectedErrString := "expected bucket dir to have trailing slash"

	testUploadBadObject := UploadObject{
		PathToFile: pathToTestFile,
		S3FileName: s3FileName,
		BucketDir:  "badbucketdir",
		Bucket:     bucket,
		Timeout:    timeout,
		NumWorkers: 5,
		PartSize:   50,
		Manipulate: true,
	}

	prefix := util.GetKeyType(policy, time.Now())
	_, err := UploadFile(svc, testUploadBadObject, prefix, false)
	if err != nil && strings.Contains(err.Error(), expectedErrString) {
		// Pass
	} else {
		t.Error("expected error due to bucket dir not including a trailing slash")
	}
}

// Test 7 - Negative Upload Testing
//	Upload a file with negative workers

func TestUploadInvalidWorkers(t *testing.T) {
	expectedErrString := "concurrent workers should not be less than 1"

	testUploadBadObject := UploadObject{
		PathToFile: pathToTestFile,
		S3FileName: s3FileName,
		BucketDir:  "",
		Bucket:     bucket,
		Timeout:    timeout,
		NumWorkers: 0,
		PartSize:   50,
		Manipulate: true,
	}

	prefix := util.GetKeyType(policy, time.Now())
	_, err := UploadFile(svc, testUploadBadObject, prefix, false)
	if err != nil && strings.Contains(err.Error(), expectedErrString) {
		// Pass
	} else {
		t.Error("expected error due to invalid number of workers specified")
	}
}

// Test 8 - Negative Upload Testing
//	Upload a file with no path specified
func TestUploadNoPathToFile(t *testing.T) {
	expectedErrString := "path to file should not be empty and must include the full path to the file"

	testUploadBadObject := UploadObject{
		PathToFile: "",
		S3FileName: s3FileName,
		BucketDir:  "",
		Bucket:     bucket,
		Timeout:    timeout,
		NumWorkers: 5,
		PartSize:   50,
		Manipulate: true,
	}

	prefix := util.GetKeyType(policy, time.Now())
	_, err := UploadFile(svc, testUploadBadObject, prefix, false)
	if err != nil && strings.Contains(err.Error(), expectedErrString) {
		// Pass
	} else {
		t.Error("expected error as no path to file specified")
	}
}

// Test 9 - Negative Upload Testing
//	Upload a file with negative timeout
func TestUploadNegativeTimeout(t *testing.T) {
	expectedErrString := "timeout must not be less than 0"

	testUploadBadObject := UploadObject{
		PathToFile: pathToTestFile,
		S3FileName: s3FileName,
		BucketDir:  "",
		Bucket:     bucket,
		Timeout:    time.Second * time.Duration(-1),
		NumWorkers: 5,
		PartSize:   50,
		Manipulate: true,
	}

	prefix := util.GetKeyType(policy, time.Now())
	_, err := UploadFile(svc, testUploadBadObject, prefix, false)
	if err != nil && strings.Contains(err.Error(), expectedErrString) {
		// Pass
	} else {
		t.Error("expected error when timeout less than 0")
	}
}
