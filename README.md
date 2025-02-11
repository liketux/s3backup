# s3backup
This is a custom take on the GFS backup strategy adopted for AWS S3 which is intended to be run on a daily basis to backup objects in S3.

The implementation uploads backups to S3 in the following way:
1. A monthly backup is taken on the first day of each month. A lifecycle policy to transition monthly objects should be implemented for objects with the 'monthly_' prefix. This utility does not handle rotation of monthly backups.
2. A weekly backup is taken every Monday (unless it's a monthly backup) with the prefix 'weekly_'. The maximum number of weekly backups kept by default is 4. When another weekly backup is created, the oldest weekly backup is rotated.
3. A daily backup is taken once a day (unless it's a monthly or weekly backup) with the prefix 'daily_'. The maximum number of daily backups kept by default is 6. This ensures that 7 daily backups are kept as a weekly backup taken on Monday.

## CLI Arguments
./s3backup -h
```
Options:
  --action   (required)     The intended action for the tool to run [backup|upload|download|rotate]
  --region   (required)     The AWS region to upload the specified file to
  --bucket   (required)     The S3 bucket to upload the specified file to
  --endpoint                The S3 endpoint amazonaws.com, storage.yandexcloud.net, etc. [default: amazonaws.com]
  --credfile                The full path to the AWS CLI credential file if environment variables are not being used to provide the access id and key
  --profile                 The profile to use for the AWS CLI credential file [default: default]
  --pathtofile              The full path to the file to upload to the specified S3 bucket. Must be specified unless --rotateonly=true
  --s3filename              The name of the file as it should appear in the S3 bucket. Must be specified unless --rotateonly=true
  --bucketdir               The directory chain in the bucket in which to upload the S3 object to. Must include the trailing slash
  --timeout                 The timeout to upload the specified file (seconds) [default: 3600]
  --dryrun                  If enabled then no upload or rotation actions will be executed [default: false]
  --concurrentworkers       The number of threads to use when uploading the file to S3 [default: 5]
  --partsize                The part size to use when performing a multipart upload or download (MB) [default: 50]
  --enforceretentionperiod  If enabled then objects in the S3 bucket will only be rotated if they are older then the retention period [default: true]
  --dailyretentioncount     The number of daily objects to keep in S3 [default: 6]
  --dailyretentionperiod    The retention period (hours) that a daily object should be kept in S3 [default: 168]
  --weeklyretentioncount    The number of weekly objects to keep in S3 [default: 4]
  --weeklyretentionperiod   The retention period (hours) that a weekly object should be kept in S3 [default: 672]
```                     
## Examples

### Backups
#### Basic Usage
```sh
./s3backup --action=backup --credfile=/backupuser/.aws_creds --region=us-east-1 --bucket=mybucket --s3filename=portfolioAlbum --pathtofile=/var/tmp/uploads/portfolioAlbum2007.tar
```

#### Usage Custom Rotation Policy (10 daily backups, 5 weekly backups with enforced retention period applied)
```sh
./s3backup --action=backup --credfile=/backupuser/.aws_creds --region=us-east-1 --bucket=mybucket --s3filename=portfolioAlbum --pathtofile=/var/tmp/uploads/portfolioAlbum2007.tar --enforceretentionperiod=true --dailyretentioncount=10 --dailyretentionperiod=240 --weeklyretentioncount=5 --weeklyretentionperiod=120
```

#### Usage with 5 hour timeout
```sh
./s3backup --action=backup --credfile=/backupuser/.aws_creds --region=us-east-1 --bucket=mybucket --s3filename=portfolioAlbum --pathtofile=/var/tmp/uploads/portfolioAlbum2007.tar --timeout=18000
```

#### Dry run
```sh
./s3backup --action=backup --credfile=/backupuser/.aws_creds --region=us-east-1 --bucket=mybucket --s3filename=portfolioAlbum --pathtofile=/var/tmp/uploads/portfolioAlbum2007.tar --dryrun=true
```

### Uploading
#### Basic Usage
```sh
./s3backup --action=upload --credfile=/backupuser/.aws_creds --region=us-east-1 --bucket=mybucket --s3filename=myFileNameThatWontChangeInBucket --pathtofile=/var/tmp/uploads/portfolioAlbum2007.tar
```

### Rotation Only
#### Basic Usage
```sh
./s3backup --action=rotate --credfile=/backupuser/.aws_creds --region=us-east-1 --bucket=mybucket --s3filename=portfolioAlbum --pathtofile=/var/tmp/uploads/portfolioAlbum2007.tar
```

### Download
#### Basic Usage
```sh
./s3backup --action=download --credfile=/backupuser/.aws_creds --region=us-east-1 --bucket=mybucket --s3filename=portfolioAlbumInS3 --pathtofile=/var/tmp/uploads/mydownloadedPortfolioAlbum
```


If you prefer, you may set environment variables instead of using a credential file:
```
AWS_ACCESS_KEY_ID=<access key id>
AWS_SECRET_ACCESS_KEY=<secret access key>
```

## Recommendations
1. This tool should be used with a lifecycle policy which moves objects to IA/Glacier to reduce costs of infrequently accessed objects. i.e. move to Glacier after 30 days
2. Replication between another bucket should be enabled for a greater level of redundancy. This is only if you are not constrained to a particular geographic location.


## Notes About Behaviour
1. An incomplete multipart upload object will be left in the S3 bucket if the upload fails due to a timeout. A policy should be set on the bucket to remove multipart upload objects after a certain period of time.
2. In addition to the 'daily_', 'weekly_', 'monthly_' prefix, a timestamp will be added as a suffix (i.e. 20170115T002115) to any file uploaded using the backup option.

## Limitations
1. The progress tracking implemented for uploads is only to provide a rough idea of how the upload is progressing. This is due to:
    * Limitations with S3 manager progress tracking
    * The progress tracking checks the amount of upload parts which is limited to 1000 per request

## Testing

Run test suite with `go test -v ./...` in base directory of repository. Testing requires the following environment variables to be set:


If you're providing credentials via file:
```
AWS_CRED_FILE=<Path to AWS credential file>
AWS_PROFILE=<AWS profile>
AWS_REGION=<AWS region where the buckets for testing exist>
AWS_BUCKET_UPLOAD=<AWS bucket specifically for upload testing>
AWS_BUCKET_ROTATION=<AWS bucket specifically for rotation testing>
AWS_BUCKET_FORBIDDEN=<AWS bucket that user running tests does not have permission to access>
AWS_BUCKET_DOWNLOAD=<AWS bucket specifically for download testing>
```

If you're providing credentials via env:

```
AWS_ACCESS_KEY_ID=<AWS access key ID>
AWS_SECRET_ACCESS_KEY=<AWS secret access key>
AWS_REGION=<AWS region where the buckets for testing exist>
AWS_ENDPOINT=<S3 enppoint>
AWS_BUCKET_UPLOAD=<AWS bucket specifically for upload testing>
AWS_BUCKET_ROTATION=<AWS bucket specifically for rotation testing>
AWS_BUCKET_FORBIDDEN=<AWS bucket that user running tests does not have permission to access>
AWS_BUCKET_DOWNLOAD=<AWS bucket specifically for download testing>
```
