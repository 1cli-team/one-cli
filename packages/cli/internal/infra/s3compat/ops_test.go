package s3compat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

type fakeBucketClient struct {
	headErr      error
	createErr    error
	headCalls    int
	createCalls  int
	createBucket string
	createConfig bool
}

func (f *fakeBucketClient) HeadBucket(context.Context, *s3.HeadBucketInput, ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	f.headCalls++
	if f.headErr != nil {
		return nil, f.headErr
	}
	return &s3.HeadBucketOutput{}, nil
}

func (f *fakeBucketClient) CreateBucket(_ context.Context, in *s3.CreateBucketInput, _ ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	f.createCalls++
	if in != nil && in.Bucket != nil {
		f.createBucket = *in.Bucket
	}
	f.createConfig = in != nil && in.CreateBucketConfiguration != nil
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &s3.CreateBucketOutput{}, nil
}

type fakeObjectClient struct {
	input *s3.PutObjectInput
}

func (f *fakeObjectClient) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.input = in
	return &s3.PutObjectOutput{}, nil
}

func TestEnsureBucketSkipsCreateWhenBucketExists(t *testing.T) {
	client := &fakeBucketClient{}
	if err := ensureBucket(context.Background(), client, &Endpoint{Bucket: "demo-abc123"}, "demo-abc123"); err != nil {
		t.Fatalf("ensureBucket: %v", err)
	}
	if client.headCalls != 1 || client.createCalls != 0 {
		t.Fatalf("calls: head=%d create=%d, want head=1 create=0", client.headCalls, client.createCalls)
	}
}

func TestEnsureBucketCreatesWhenMissing(t *testing.T) {
	client := &fakeBucketClient{
		headErr: &smithy.GenericAPIError{Code: "NoSuchBucket", Message: "Volume not found"},
	}
	err := ensureBucket(context.Background(), client, &Endpoint{
		Endpoint: "http://127.0.0.1:9000",
		Region:   "us-east-1",
		Bucket:   "demo-abc123",
	}, "demo-abc123")
	if err != nil {
		t.Fatalf("ensureBucket: %v", err)
	}
	if client.headCalls != 1 || client.createCalls != 1 {
		t.Fatalf("calls: head=%d create=%d, want head=1 create=1", client.headCalls, client.createCalls)
	}
	if client.createBucket != "demo-abc123" {
		t.Fatalf("created bucket = %q, want demo-abc123", client.createBucket)
	}
	if client.createConfig {
		t.Fatal("custom endpoint create should not include AWS LocationConstraint")
	}
}

func TestEnsureBucketTreatsAlreadyOwnedAsSuccess(t *testing.T) {
	client := &fakeBucketClient{
		headErr:   &smithy.GenericAPIError{Code: "NoSuchBucket", Message: "not found"},
		createErr: &smithy.GenericAPIError{Code: "BucketAlreadyOwnedByYou", Message: "already owned"},
	}
	err := ensureBucket(context.Background(), client, &Endpoint{Bucket: "demo-abc123"}, "demo-abc123")
	if err != nil {
		t.Fatalf("ensureBucket: %v", err)
	}
	if client.headCalls != 1 || client.createCalls != 1 {
		t.Fatalf("calls: head=%d create=%d, want head=1 create=1", client.headCalls, client.createCalls)
	}
}

func TestCreateBucketInputSetsAWSRegionConstraint(t *testing.T) {
	in := createBucketInput(&Endpoint{Region: "ap-southeast-1"}, "demo-abc123")
	if in.CreateBucketConfiguration == nil {
		t.Fatal("AWS non-us-east-1 create should include LocationConstraint")
	}
	if got := string(in.CreateBucketConfiguration.LocationConstraint); got != "ap-southeast-1" {
		t.Fatalf("LocationConstraint = %q, want ap-southeast-1", got)
	}

	in = createBucketInput(&Endpoint{Endpoint: "http://127.0.0.1:9000", Region: "ap-southeast-1"}, "demo-abc123")
	if in.CreateBucketConfiguration != nil {
		t.Fatal("custom S3-compatible endpoint should not include AWS LocationConstraint")
	}
}

func TestEnsureBucketBucketAlreadyExistsIsActionable(t *testing.T) {
	client := &fakeBucketClient{
		headErr:   &smithy.GenericAPIError{Code: "NoSuchBucket", Message: "not found"},
		createErr: &smithy.GenericAPIError{Code: "BucketAlreadyExists", Message: "bucket exists"},
	}
	err := ensureBucket(context.Background(), client, &Endpoint{
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "demo-abc123",
	}, "demo-abc123")

	got, ok := err.(*output.Error)
	if !ok {
		t.Fatalf("error type = %T, want *output.Error", err)
	}
	if !strings.Contains(got.Message, `S3 bucket "demo-abc123" 已存在但不属于当前账号`) {
		t.Fatalf("message should explain bucket name collision, got %q", got.Message)
	}
	if got.Context["bucket"] != "demo-abc123" {
		t.Fatalf("context.bucket = %v, want demo-abc123", got.Context["bucket"])
	}
	if len(got.Remediation) != 2 {
		t.Fatalf("remediation count = %d, want 2: %#v", len(got.Remediation), got.Remediation)
	}
	if got.Remediation[0].Action != "change-s3-bucket" || got.Remediation[1].Action != "use-owned-bucket" {
		t.Fatalf("unexpected remediation: %#v", got.Remediation)
	}
}

func TestEnsureBucketCreateFailureIsActionable(t *testing.T) {
	client := &fakeBucketClient{
		headErr:   &smithy.GenericAPIError{Code: "NoSuchBucket", Message: "Volume not found"},
		createErr: &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied"},
	}
	err := ensureBucket(context.Background(), client, &Endpoint{
		Endpoint: "http://127.0.0.1:9000",
		Bucket:   "demo-abc123",
	}, "demo-abc123")

	got, ok := err.(*output.Error)
	if !ok {
		t.Fatalf("error type = %T, want *output.Error", err)
	}
	if !strings.Contains(got.Message, `S3 bucket "demo-abc123" 不存在，自动创建失败`) {
		t.Fatalf("message should explain create failure, got %q", got.Message)
	}
	if len(got.Remediation) != 2 {
		t.Fatalf("remediation count = %d, want 2: %#v", len(got.Remediation), got.Remediation)
	}
}

func TestUploadErrorNoSuchBucketIsActionable(t *testing.T) {
	err := uploadError(&smithy.GenericAPIError{
		Code:    "NoSuchBucket",
		Message: "Volume not found",
	}, &Endpoint{
		Endpoint: "http://127.0.0.1:9000",
		Region:   "us-east-1",
		Bucket:   "demo-abc123",
	}, "404.html")

	got, ok := err.(*output.Error)
	if !ok {
		t.Fatalf("error type = %T, want *output.Error", err)
	}
	if !strings.Contains(got.Message, `S3 bucket "demo-abc123" 不存在`) {
		t.Fatalf("message should name the missing bucket, got %q", got.Message)
	}
	if got.Context["bucket"] != "demo-abc123" {
		t.Fatalf("context.bucket = %v, want demo-abc123", got.Context["bucket"])
	}
	if len(got.Remediation) != 2 {
		t.Fatalf("remediation count = %d, want 2: %#v", len(got.Remediation), got.Remediation)
	}
	if got.Remediation[0].Action != "create-s3-bucket" {
		t.Fatalf("first remediation = %#v", got.Remediation[0])
	}
	if !strings.Contains(got.Remediation[0].Command, "--bucket demo-abc123") ||
		!strings.Contains(got.Remediation[0].Command, "--endpoint-url http://127.0.0.1:9000") {
		t.Fatalf("create command missing bucket or endpoint: %q", got.Remediation[0].Command)
	}
}

func TestUploadOneSetsContentLengthAndLeavesChecksumUnset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.js")
	payload := []byte("console.log('ok')\n")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	client := &fakeObjectClient{}
	err := uploadOne(context.Background(), client, "demo-abc123", uploadFile{
		relPath: "assets/app.js",
		absPath: path,
	})
	if err != nil {
		t.Fatalf("uploadOne: %v", err)
	}
	if client.input == nil {
		t.Fatal("PutObject was not called")
	}
	if client.input.ContentLength == nil || *client.input.ContentLength != int64(len(payload)) {
		t.Fatalf("ContentLength = %v, want %d", client.input.ContentLength, len(payload))
	}
	if string(client.input.ChecksumAlgorithm) != "" {
		t.Fatalf("ChecksumAlgorithm = %q, want unset", client.input.ChecksumAlgorithm)
	}
	if client.input.ContentType == nil || *client.input.ContentType != "text/javascript; charset=utf-8" {
		t.Fatalf("ContentType = %v, want JavaScript MIME", client.input.ContentType)
	}
}
