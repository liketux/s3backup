package main

import (
	"github.com/alexflint/go-arg"
	"github.com/aws/aws-sdk-go/service/s3"
	"s3backup/download"
	"s3backup/log"
	"s3backup/rotate"
	"s3backup/rpolicy"
	"s3backup/s3client"
	"s3backup/upload"
	"s3backup/util"
	"os"
	"strconv"
	"time"
)

type args struct {
	Action                 string `arg:"help:The intended action for the tool to run [backup|upload|download|rotate]"`
	Region                 string `arg:"required,help:The AWS region to upload the specified file to"`
	Bucket                 string `arg:"required,help:The S3 bucket to upload the specified file to"`
	CredFile               string `arg:"help:The full path to the AWS CLI credential file if environment variables are not being used to provide the access id and key"`
	Profile                string `arg:"help:The profile to use for the AWS CLI credential file"`
	PathToFile             string `arg:"help:The full path to the file to upload to the specified S3 bucket. Must be specified unless --rotateonly=true"`
	S3FileName             string `arg:"help:The name of the file as it should appear in the S3 bucket. Must be specified unless --rotateonly=true"`
	BucketDir              string `arg:"help:The directory chain in the bucket in which to upload the S3 object to. Must include the trailing slash"`
	Endpoint               string `arg:"help:s3 provider endpoint amazonaws.com or storage.yandexcloud.net"`
	Timeout                int    `arg:"help:The timeout to upload the specified file (seconds)"`
	DryRun                 bool   `arg:"help:If enabled then no upload or rotation actions will be executed [default: false]"`
	ConcurrentWorkers      int    `arg:"help:The number of threads to use when uploading the file to S3"`
	PartSize               int    `arg:"help:The part size to use when performing a multipart upload or download (MB)"`
	EnforceRetentionPeriod bool   `arg:"help:If enabled then objects in the S3 bucket will only be rotated if they are older then the retention period"`
	DailyRetentionCount    int    `arg:"help:The number of daily objects to keep in S3"`
	DailyRetentionPeriod   int    `arg:"help:The retention period (hours) that a daily object should be kept in S3"`
	WeeklyRetentionCount   int    `arg:"help:The number of weekly objects to keep in S3"`
	WeeklyRetentionPeriod  int    `arg:"help:The retention period (hours) that a weekly object should be kept in S3"`
}

func init() {
	log.Init(os.Stdout, os.Stdout, os.Stderr)
}

func main() {
	// Set default args
	args := args{}
	args.Timeout = 3600 // Default timeout to 1 hour for file upload
	args.CredFile = util.GetEnvString("AWS_CRED_FILE", "")
	args.Profile = util.GetEnvString("AWS_PROFILE", "default")
	args.BucketDir = util.GetEnvString("AWS_BUCKET", "")
	args.Endpoint = util.GetEnvString("AWS_ENDPOINT", "amazonaws.com")
	args.EnforceRetentionPeriod = true
	args.DryRun = false
	args.ConcurrentWorkers = 5
	args.PartSize = 50
	args.DailyRetentionCount = 6
	args.DailyRetentionPeriod = 168
	args.WeeklyRetentionCount = 4
	args.WeeklyRetentionPeriod = 672

	// Parse args from command line
	arg.MustParse(&args)

	logArgs(args)

	log.Info.Println(`
	######################################
	#        s3backup started            #
	######################################
	`)

	svc, err := s3client.CreateS3Client(args.CredFile, args.Profile, args.Region, args.Endpoint)
	if err != nil {
		log.Error.Println(err)
		os.Exit(1)
	}

	runAction(svc, args)

	log.Info.Println("Finished s3backup!")

	log.Info.Println(`
	######################################
	#      s3backup finished             #
	######################################
	`)

}

func runAction(svc *s3.S3, args args) {
	switch args.Action {
	case "backup":
		runBackupAction(svc, args)
	case "upload":
		runUploadAction(svc, args)
	case "download":
		runDownloadAction(svc, args)
	case "rotate":
		runRotateAction(svc, args)
	default:
		log.Error.Println("unexpected action specified: " + args.Action)
	}
}

func runBackupAction(svc *s3.S3, arguments args) {
	log.Info.Println("Backup action specified, backing up file")

	rotationPolicy := getRotationPolicy(arguments)

	log.Info.Println("Starting standard GFS upload and rotation")
	prefix := util.GetKeyType(rotationPolicy, time.Now())
	_, err := upload.UploadFile(svc, getUploadObject(arguments, true), prefix, arguments.DryRun)
	if err != nil {
		log.Error.Printf("Failed to upload file. Aborting backup. Reason: %v\n", err)
		os.Exit(1)
	}

	rotate.StartRotation(svc, arguments.Bucket, rotationPolicy, arguments.BucketDir, arguments.DryRun)
	log.Info.Println("Upload and Rotation Complete!")

}

func runUploadAction(svc *s3.S3, arguments args) {
	log.Info.Println("Upload action specified, uploading file")

	_, err := upload.UploadFile(svc, getUploadObject(arguments, false), "", arguments.DryRun)
	if err != nil {
		log.Error.Printf("Failed to upload file. Reason: %v\n", err)
		os.Exit(1)
	}
}

func runRotateAction(svc *s3.S3, arguments args) {
	log.Info.Println("Rotate action specified, proceeding with rotation only")
	rotate.StartRotation(svc, arguments.Bucket, getRotationPolicy(arguments), arguments.BucketDir, arguments.DryRun)
}

func runDownloadAction(svc *s3.S3, arguments args) {
	log.Info.Println("Download action specified, downloading file")

	downloadObject := download.DownloadObject{
		DownloadLocation: arguments.PathToFile,
		S3FileKey:        arguments.S3FileName,
		BucketDir:        arguments.BucketDir,
		Endpoint:         arguments.Endpoint,
		Bucket:           arguments.Bucket,
		NumWorkers:       arguments.ConcurrentWorkers,
		PartSize:         arguments.PartSize,
	}
	err := download.DownloadFile(svc, downloadObject)
	if err != nil {
		log.Error.Printf("Failed to download file. Aborting. Reason: %v\n", err)
		os.Exit(1)
	}

}

func getUploadObject(arguments args, manipulate bool) upload.UploadObject {
	return upload.UploadObject{
		PathToFile: arguments.PathToFile,
		S3FileName: arguments.S3FileName,
		BucketDir:  arguments.BucketDir,
		Endpoint:   arguments.Endpoint,
		Bucket:     arguments.Bucket,
		Timeout:    time.Second * time.Duration(arguments.Timeout),
		NumWorkers: arguments.ConcurrentWorkers,
		PartSize:   arguments.PartSize,
		Manipulate: manipulate,
	}
}

func getRotationPolicy(arguments args) rpolicy.RotationPolicy {
	if !arguments.EnforceRetentionPeriod {
		log.Warn.Println("s3backup is running with enforce retention period disabled. " +
			"This may result in objects being deleted that which have not exceeded the retention period")
	}

	//  Standard GFS rotation policy
	return rpolicy.RotationPolicy{
		DailyRetentionPeriod: time.Hour * time.Duration(arguments.DailyRetentionPeriod),
		DailyRetentionCount:  arguments.DailyRetentionCount,
		DailyPrefix:          "daily_",

		WeeklyRetentionPeriod: time.Hour * time.Duration(arguments.WeeklyRetentionPeriod),
		WeeklyRetentionCount:  arguments.WeeklyRetentionCount,
		WeeklyPrefix:          "weekly_",

		MonthlyPrefix:          "monthly_",
		EnforceRetentionPeriod: arguments.EnforceRetentionPeriod,
	}

}

func logArgs(arguments args) {
	log.Info.Println("Loaded s3backup with arguments: ")

	log.Info.Println("--credfile=" + arguments.CredFile)
	log.Info.Println("--region=" + arguments.Region)
	log.Info.Println("--bucket=" + arguments.Bucket)
	log.Info.Println("--bucketdir=" + arguments.BucketDir)
	log.Info.Println("--endpoint=" + arguments.Endpoint)
	log.Info.Println("--profile=" + arguments.Profile)
	log.Info.Println("--action=" + arguments.Action)
	log.Info.Println("--pathtofile=" + arguments.PathToFile)
	log.Info.Println("--s3filename=" + arguments.S3FileName)
	log.Info.Println("--dryrun=" + strconv.FormatBool(arguments.DryRun))
	log.Info.Println("--timeout=" + strconv.Itoa(arguments.Timeout))
	log.Info.Println("--enforceretentionperiod=" + strconv.FormatBool(arguments.EnforceRetentionPeriod))
	log.Info.Println("--concurrentworkers=" + strconv.Itoa(arguments.ConcurrentWorkers))
	log.Info.Println("--partsize=" + strconv.Itoa(arguments.PartSize))
	log.Info.Println("--dailyretentioncount=" + strconv.Itoa(arguments.DailyRetentionCount))
	log.Info.Println("--dailyretentionperiod=" + strconv.Itoa(arguments.DailyRetentionPeriod))
	log.Info.Println("--weeklyretentioncount=" + strconv.Itoa(arguments.WeeklyRetentionCount))
	log.Info.Println("--weeklyretentionperiod=" + strconv.Itoa(arguments.WeeklyRetentionPeriod))

}
