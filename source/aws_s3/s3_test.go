package awss3

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang-migrate/migrate/v4/source"
	st "github.com/golang-migrate/migrate/v4/source/testing"
)

func Test(t *testing.T) {
	s3Client := fakeS3{
		bucket: "some-bucket",
		objects: map[string]string{
			"staging/migrations/1_foobar.up.sql":          "1 up",
			"staging/migrations/1_foobar.down.sql":        "1 down",
			"prod/migrations/1_foobar.up.sql":             "1 up",
			"prod/migrations/1_foobar.down.sql":           "1 down",
			"prod/migrations/3_foobar.up.sql":             "3 up",
			"prod/migrations/4_foobar.up.sql":             "4 up",
			"prod/migrations/4_foobar.down.sql":           "4 down",
			"prod/migrations/5_foobar.down.sql":           "5 down",
			"prod/migrations/7_foobar.up.sql":             "7 up",
			"prod/migrations/7_foobar.down.sql":           "7 down",
			"prod/migrations/not-a-migration.txt":         "",
			"prod/migrations/0-random-stuff/whatever.txt": "",
		},
	}
	driver := s3Driver{
		bucket:     "some-bucket",
		prefix:     "prod/migrations/",
		migrations: source.NewMigrations(),
		s3client:   &s3Client,
	}
	err := driver.loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	st.Test(t, &driver)
}

func TestSetAWSSession(t *testing.T) {
	customSession, err := session.NewSession(&aws.Config{
		Endpoint:         aws.String("http://localhost:4572"),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	SetAWSSession(customSession)
	if customSession != awsSession {
		t.Error("Custom AWS session was not saved")
	}
}

func TestNewS3Driver(t *testing.T) {
	const expectedBucket = "migration-bucket"
	const expectedPrefix = "production/"

	driver, err := newS3Driver(fmt.Sprintf("s3://%s/%s", expectedBucket, expectedPrefix))
	if err != nil {
		t.Fatal(err)
	}

	if driver.bucket != expectedBucket {
		t.Errorf("Expected: %s; actual: %s", expectedBucket, driver.bucket)
	}

	if driver.prefix != expectedPrefix {
		t.Errorf("Expected: %s; actual: %s", expectedPrefix, driver.prefix)
	}

	if driver.s3client == nil {
		t.Error("S3 client is not initialized")
	}

	if driver.migrations == nil {
		t.Error("Migration source is not initialized")
	}

	driver, err = newS3Driver(fmt.Sprintf("s3://%s", expectedBucket))
	if err != nil {
		t.Fatal(err)
	}

	if driver.bucket != expectedBucket {
		t.Errorf("Expected: %s; actual: %s", expectedBucket, driver.bucket)
	}

	if driver.prefix != "" {
		t.Errorf("Prefix should be empty")
	}
}

type fakeS3 struct {
	s3.S3
	bucket  string
	objects map[string]string
}

func (s *fakeS3) ListObjects(input *s3.ListObjectsInput) (*s3.ListObjectsOutput, error) {
	bucket := aws.StringValue(input.Bucket)
	if bucket != s.bucket {
		return nil, errors.New("bucket not found")
	}
	prefix := aws.StringValue(input.Prefix)
	delimiter := aws.StringValue(input.Delimiter)
	var output s3.ListObjectsOutput
	for name := range s.objects {
		if strings.HasPrefix(name, prefix) {
			if delimiter == "" || !strings.Contains(strings.Replace(name, prefix, "", 1), delimiter) {
				output.Contents = append(output.Contents, &s3.Object{
					Key: aws.String(name),
				})
			}
		}
	}
	return &output, nil
}

func (s *fakeS3) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	bucket := aws.StringValue(input.Bucket)
	if bucket != s.bucket {
		return nil, errors.New("bucket not found")
	}
	if data, ok := s.objects[aws.StringValue(input.Key)]; ok {
		body := ioutil.NopCloser(strings.NewReader(data))
		return &s3.GetObjectOutput{Body: body}, nil
	}
	return nil, errors.New("object not found")
}
