package util

import (
	"crypto/md5"
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"s3backup/rpolicy"
	"s3backup/s3client"
	"github.com/jinzhu/now"
	"io"
	"os"
	"regexp"
	"time"
	"strconv"
)

// CheckPrefix checks if the prefix of a string matches the specified prefix.
// Returns true if it matches; else false
func CheckPrefix(key string, prefix string) bool {
	re := regexp.MustCompile("^" + prefix)
	return re.Match([]byte(key))
}

// CleanUpMultiPartUploads is a Helpful function to get rid of all abandoned multipart uploads
func CleanUpMultiPartUploads(svc *s3.S3, bucket string) error {
	multiPartUploads, err := s3client.GetAllMultiPartUploads(svc, bucket)
	if err != nil {
		return err
	}
	for key, uploadId := range multiPartUploads {
		s3client.AbortAllMultiPartUploads(svc, bucket, key, uploadId)
	}
	return nil
}

// RetrieveSortedKeysByTime is a helper function to get all sorted keys
func RetrieveSortedKeysByTime(svc *s3.S3, bucket string, prefix string, bucketDir string) ([]s3client.BucketEntry, error) {
	keys, err := s3client.GetKeysByPrefix(svc, bucket, bucketDir + prefix)
	if err != nil {
		return nil, err
	}

	numKeys := len(keys)
	if numKeys == 0 {
		return nil, nil
	}
	return s3client.SortKeysByTime(keys), nil
}

// GetKeyType returns the specified key type (_monthly, _weekly, _daily) for a particular time
func GetKeyType(policy rpolicy.RotationPolicy, keyTime time.Time) string {
	monthlyYear, monthlyMonth, monthlyDay := now.New(keyTime).BeginningOfMonth().Date()

	keyTimeYear, keyTimeMonth, keyTimeDay := keyTime.Date()

	if keyTimeYear == monthlyYear && monthlyMonth == keyTimeMonth && monthlyDay == keyTimeDay {
		// This is a monthly backup as it falls on the first day of the month
		return policy.MonthlyPrefix
	}

	if keyTime.Weekday() == time.Monday {
		// This is a weekly backup as it falls on a Monday
		return policy.WeeklyPrefix
	}

	// Every other backup will be daily
	return policy.DailyPrefix
}

// FindKeyInBucket returns true if the specified key exists in the *s3.ListObjectOutput; otherwise false
func FindKeyInBucket(keyToFind string, bucketContents *s3.ListObjectsOutput) bool {
	for _, key := range bucketContents.Contents {
		if *key.Key == keyToFind {
			return true
		}
	}
	return false
}

// FindKeysInBucketByPrefix will return all the keys that match a prefix in the provided *s3.ListObjectsOutput
func FindKeysInBucketByPrefix(prefix string, bucketContents *s3.ListObjectsOutput) []string {
	keys := []string{}
	for _, key := range bucketContents.Contents {
		if CheckPrefix(*key.Key, prefix) {
			keys = append(keys, *key.Key)
		}
	}
	return keys
}

// EmptyBucket simply deletes all the objects in the specified bucket
func EmptyBucket(svc *s3.S3, bucket string) error {
	result, err := s3client.GetBucketContents(svc, bucket)
	if err != nil {
		return err
	}
	for _, key := range result.Contents {
		_, err := svc.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(*key.Key),
		})
		if err != nil {
			return err
		}
	}
	result, err = s3client.GetBucketContents(svc, bucket)

	if err != nil {
		return err
	}
	if len(result.Contents) > 0 {
		return errors.New("expected bucket contents to be 0 after emptying")
	}

	return nil
}

// CheckBucketSize returns true if the bucket size is the same as the expected bucket size; else false
func CheckBucketSize(bucketContents *s3.ListObjectsOutput, expectedContentSize int) bool {

	bucketContentsLength := len(bucketContents.Contents)

	if bucketContentsLength != expectedContentSize {
		return false
	}
	return true

}

// CreateBigFile writes a file to disk that consists of null characters
func CreateBigFile(pathToBigFile string, size int64) error {
	fd, err := os.Create(pathToBigFile)
	defer fd.Close()

	if err != nil {
		return err
	}
	_, err = fd.Seek(size-1, 0)
	if err != nil {
		return err
	}
	_, err = fd.Write([]byte{0}) // Write 500MB worth of null bits to the file
	if err != nil {
		return err
	}
	return nil
}

// CreateFile writes the provided byte array to a file on disk
func CreateFile(pathToFile string, contents []byte) error {
	fd, err := os.Create(pathToFile)
	defer fd.Close()

	fd.Write(contents)

	if err != nil {
		return err
	}
	return nil
}

// ComputeMD5Sum takes the full path of a file and returns the md5sum
func ComputeMD5Sum(filePath string) ([]byte, error) {
	var result []byte
	file, err := os.Open(filePath)
	if err != nil {
		return result, err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return result, err
	}

	return hash.Sum(result), nil
}


// GetEnvString gets the environment variable for a key and if that env-var hasn't been set it returns the default value
func GetEnvString(key string, defaultVal string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		value = defaultVal
	}
	return value
}

// GetEnvBool gets the environment variable for a key and if that env-var hasn't been set it returns the default value
func GetEnvBool(key string, defaultVal bool) (bool, error) {
	envvalue := os.Getenv(key)
	if len(envvalue) == 0 {
		value := defaultVal
		return value, nil
	}

	value, err := strconv.ParseBool(envvalue)
	return value, err
}

// GetEnvInt gets the environment variable for a key and if that env-var hasn't been set it returns the default value. This function is equivalent to ParseInt(s, 10, 0) to convert env-vars to type int
func GetEnvInt(key string, defaultVal int) (int, error) {
	envvalue := os.Getenv(key)
	if len(envvalue) == 0 {
		value := defaultVal
		return value, nil
	}

	value, err := strconv.Atoi(envvalue)
	return value, err
}

// GetEnvFloat gets the environment variable for a key and if that env-var hasn't been set it returns the default value. This function uses bitSize of 64 to convert string to float64.
func GetEnvFloat(key string, defaultVal float64) (float64, error) {
	envvalue := os.Getenv(key)
	if len(envvalue) == 0 {
		value := defaultVal
		return value, nil
	}

	value, err := strconv.ParseFloat(envvalue, 64)
	return value, err
}
