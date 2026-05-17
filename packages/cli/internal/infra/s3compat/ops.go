package s3compat

// ops.go exposes the Render / Apply operations as ordinary package
// functions with package-local types. provider.go adapts these to
// deploy.Provider for each of the six S3-compatible backend ids.

import (
	"context"
	stderrors "errors"
	"fmt"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

// defaultBuildOutput is the conventional Vite/CRA/Astro output dir.
// Profile / per-subproject extensions can override later if needed.
const defaultBuildOutput = "dist"

// Endpoint is the s3-relevant slice of a deploy profile. Single shape
// covers AWS S3 / Aliyun OSS / Tencent COS / MinIO / RustFS /
// Cloudflare R2 — the Endpoint URL + region discriminate the vendor;
// the active backend id is owned by the manifest, not this struct.
type Endpoint struct {
	Endpoint       string
	Region         string
	Bucket         string
	ForcePathStyle bool
}

// Credentials is the AccessKey pair (same shape across all S3-protocol
// vendors).
type Credentials struct {
	AccessKeyID     string
	AccessKeySecret string
}

// Subproject is the per-subproject context Render / Apply need.
type Subproject struct {
	Name        string
	RelativeDir string
	Toolchain   string
}

// Stable JSON envelope schema string for the s3 apply operation.
const SchemaApply = "one-cli/deploy-apply/v1"

// ApplyInput addresses Apply.
type ApplyInput struct {
	ProjectRoot string
	Subproject  *Subproject
	Endpoint    *Endpoint
	Credentials *Credentials
	DryRun      bool
	// Kind is the bare backend id ("aliyun-oss" / "aws-s3" / ...). Used
	// only in error messages so the user sees the same id they typed
	// at `one configure add deploy/<kind>`.
	Kind string
}

// ApplyResult is the Apply envelope.
type ApplyResult struct {
	Schema string   `json:"schema"`
	Argv   []string `json:"argv"`
	DryRun bool     `json:"dry_run"`
}

type bucketClient interface {
	HeadBucket(context.Context, *s3.HeadBucketInput, ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
	CreateBucket(context.Context, *s3.CreateBucketInput, ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
}

// Apply builds and uploads the configured subproject's static output.
func Apply(ctx context.Context, in ApplyInput) (*ApplyResult, error) {
	kindLabel := in.Kind
	if kindLabel == "" {
		kindLabel = "aws-s3"
	}
	if in.Subproject == nil {
		return nil, cliErrors.New(cliErrors.ONE_CLI_ERROR,
			fmt.Sprintf("deploy/%s Apply 需要 Subproject 上下文（per-subproject scope）", kindLabel))
	}
	if in.Endpoint == nil {
		return nil, cliErrors.New(cliErrors.PROFILE_NONE_CONFIGURED,
			fmt.Sprintf("deploy/%s 缺少 endpoint 配置；先 `one configure add deploy/%s <name> ...`", kindLabel, kindLabel))
	}
	if in.Credentials == nil ||
		strings.TrimSpace(in.Credentials.AccessKeyID) == "" ||
		strings.TrimSpace(in.Credentials.AccessKeySecret) == "" {
		return nil, cliErrors.New(cliErrors.PROFILE_NONE_CONFIGURED,
			fmt.Sprintf("deploy/%s 缺少 credentials；profile 没有 accessKeyId/accessKeySecret", kindLabel))
	}
	subDir := filepath.Join(in.ProjectRoot, filepath.FromSlash(in.Subproject.RelativeDir))
	outDir := filepath.Join(subDir, defaultBuildOutput)
	bucket := in.Endpoint.Bucket

	if in.DryRun {
		argv := []string{
			"s3-upload",
			"--endpoint", endpointDisplay(in.Endpoint),
			"--bucket", bucket,
			"--ensure-bucket",
			"--source", outDir,
			"--build-cmd", "pnpm run build",
		}
		return &ApplyResult{Schema: SchemaApply, Argv: argv, DryRun: true}, nil
	}

	if err := runBuild(ctx, subDir, in.Subproject.Toolchain); err != nil {
		return nil, err
	}
	files, err := walkUploadable(outDir)
	if err != nil {
		return nil, err
	}
	argv := []string{
		"s3-upload",
		"--endpoint", endpointDisplay(in.Endpoint),
		"--bucket", bucket,
		"--source", outDir,
		fmt.Sprintf("--files=%d", len(files)),
	}

	client, err := newS3Client(ctx, in.Endpoint, in.Credentials)
	if err != nil {
		return nil, err
	}
	if err := ensureBucket(ctx, client, in.Endpoint, bucket); err != nil {
		return nil, err
	}
	for _, f := range files {
		if err := uploadOne(ctx, client, bucket, f); err != nil {
			return nil, uploadError(err, in.Endpoint, f.relPath)
		}
	}
	return &ApplyResult{Schema: SchemaApply, Argv: argv, DryRun: false}, nil
}

func ensureBucket(ctx context.Context, client bucketClient, ep *Endpoint, bucket string) error {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return cliErrors.New(cliErrors.ONE_CLI_ERROR,
			"S3-compatible deploy 缺少 bucket；默认应来自 one.manifest.json#project.id。")
	}
	headInput := &s3.HeadBucketInput{Bucket: &bucket}
	if _, err := client.HeadBucket(ctx, headInput); err == nil {
		return nil
	} else if !isBucketMissing(err) {
		return bucketCheckError(err, ep, bucket)
	}

	if _, err := client.CreateBucket(ctx, createBucketInput(ep, bucket)); err != nil {
		if isBucketAlreadyOwned(err) {
			return nil
		}
		if isBucketNameTaken(err) {
			return bucketNameTakenError(err, ep, bucket)
		}
		return bucketCreateError(err, ep, bucket)
	}
	return nil
}

func createBucketInput(ep *Endpoint, bucket string) *s3.CreateBucketInput {
	in := &s3.CreateBucketInput{Bucket: &bucket}
	if ep == nil || strings.TrimSpace(ep.Endpoint) != "" {
		return in
	}
	region := strings.TrimSpace(ep.Region)
	if region == "" || region == "us-east-1" {
		return in
	}
	in.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{
		LocationConstraint: s3types.BucketLocationConstraint(region),
	}
	return in
}

func bucketCheckError(err error, ep *Endpoint, bucket string) error {
	return cliErrors.New(cliErrors.ONE_CLI_ERROR,
		fmt.Sprintf("无法检查 S3 bucket %q：%v", bucket, err)).
		WithContext(map[string]any{
			"bucket":   bucket,
			"endpoint": endpointDisplay(ep),
		}).
		WithRemediation(output.Remediation{
			Action: "check-s3-permission",
			Hint:   "确认 S3-compatible profile 的 AK/SK 有 HeadBucket 权限，且 endpoint/region 配置正确",
		})
}

func bucketCreateError(err error, ep *Endpoint, bucket string) error {
	return cliErrors.New(cliErrors.ONE_CLI_ERROR,
		fmt.Sprintf("S3 bucket %q 不存在，自动创建失败：%v", bucket, err)).
		WithContext(map[string]any{
			"bucket":   bucket,
			"endpoint": endpointDisplay(ep),
		}).
		WithRemediation(
			output.Remediation{
				Action: "check-s3-permission",
				Hint:   "确认 S3-compatible profile 的 AK/SK 有 CreateBucket 权限",
			},
			output.Remediation{
				Action: "use-existing-bucket",
				Hint:   "如果对象存储已经有别的 bucket，请在 one.manifest.json 的 projects[].deploy.bucket 写入那个 bucket 名",
			},
		)
}

func bucketNameTakenError(err error, ep *Endpoint, bucket string) error {
	return cliErrors.New(cliErrors.ONE_CLI_ERROR,
		fmt.Sprintf("S3 bucket %q 已存在但不属于当前账号，无法自动创建：%v", bucket, err)).
		WithContext(map[string]any{
			"bucket":   bucket,
			"endpoint": endpointDisplay(ep),
		}).
		WithRemediation(
			output.Remediation{
				Action: "change-s3-bucket",
				Hint:   "换一个当前账号可创建/拥有的 bucket 名",
			},
			output.Remediation{
				Action: "use-owned-bucket",
				Hint:   "如果已经有可用 bucket，请在 one.manifest.json 的 projects[].deploy.bucket 写入那个 bucket 名",
			},
		)
}

func uploadError(err error, ep *Endpoint, key string) error {
	if isNoSuchBucket(err) {
		bucket := ""
		if ep != nil {
			bucket = strings.TrimSpace(ep.Bucket)
		}
		endpoint := endpointDisplay(ep)
		return cliErrors.New(cliErrors.ONE_CLI_ERROR,
			fmt.Sprintf("S3 bucket %q 不存在，无法上传 %s。当前 bucket 默认来自 one.manifest.json#project.id；请先在该 S3 endpoint 创建 bucket，或把 projects[].deploy.bucket 改成已存在的 bucket。", bucket, key)).
			WithContext(map[string]any{
				"bucket":   bucket,
				"endpoint": endpoint,
				"key":      key,
			}).
			WithRemediation(s3BucketRemediations(ep, bucket)...)
	}
	return cliErrors.New(cliErrors.ONE_CLI_ERROR,
		fmt.Sprintf("S3 上传失败 %s：%v", key, err)).
		WithContext(map[string]any{
			"bucket":   bucketFromEndpoint(ep),
			"endpoint": endpointDisplay(ep),
			"key":      key,
		})
}

func isNoSuchBucket(err error) bool {
	return isBucketMissing(err)
}

func isBucketMissing(err error) bool {
	var apiErr smithy.APIError
	if stderrors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchBucket", "NotFound":
			return true
		}
	}
	msg := err.Error()
	return strings.Contains(msg, "NoSuchBucket") ||
		strings.Contains(msg, "Volume not found") ||
		strings.Contains(msg, "StatusCode: 404")
}

func isBucketAlreadyOwned(err error) bool {
	var apiErr smithy.APIError
	if stderrors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "BucketAlreadyOwnedByYou":
			return true
		}
	}
	msg := err.Error()
	return strings.Contains(msg, "BucketAlreadyOwnedByYou")
}

func isBucketNameTaken(err error) bool {
	var apiErr smithy.APIError
	if stderrors.As(err, &apiErr) {
		return apiErr.ErrorCode() == "BucketAlreadyExists"
	}
	return strings.Contains(err.Error(), "BucketAlreadyExists")
}

func s3BucketRemediations(ep *Endpoint, bucket string) []output.Remediation {
	steps := []output.Remediation{
		{
			Action: "create-s3-bucket",
			Hint:   fmt.Sprintf("在当前 S3/RustFS/MinIO endpoint 创建 bucket/volume：%s", bucket),
		},
		{
			Action: "use-existing-bucket",
			Hint:   "如果对象存储已经有别的 bucket，请在 one.manifest.json 的 projects[].deploy.bucket 写入那个 bucket 名",
		},
	}
	if ep != nil && strings.TrimSpace(ep.Endpoint) != "" && bucket != "" {
		cmd := fmt.Sprintf("aws s3api create-bucket --bucket %s --endpoint-url %s", bucket, strings.TrimSpace(ep.Endpoint))
		if region := strings.TrimSpace(ep.Region); region != "" {
			cmd += " --region " + region
		}
		steps[0].Command = cmd
	}
	return steps
}

func buildCommand(toolchain string) ([]string, error) {
	if toolchain != "" && toolchain != "node" {
		return nil, cliErrors.New(cliErrors.ONE_CLI_ERROR,
			fmt.Sprintf("S3-compatible deploy 当前只支持 node toolchain；该项目 toolchain=%s。请改用 deploy/kustomize 或贡献新 toolchain 支持。", toolchain))
	}
	return []string{"pnpm", "run", "build"}, nil
}

func runBuild(ctx context.Context, subDir, toolchain string) error {
	argv, err := buildCommand(toolchain)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = subDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return cliErrors.New(cliErrors.RUN_COMMAND_NOT_FOUND,
			fmt.Sprintf("`pnpm run build` 在 %s 失败：%v", subDir, err))
	}
	return nil
}

type uploadFile struct {
	relPath string
	absPath string
}

func walkUploadable(outDir string) ([]uploadFile, error) {
	info, err := os.Stat(outDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, cliErrors.New(cliErrors.ONE_CLI_ERROR,
				fmt.Sprintf("构建产物不存在：%s（构建步骤是不是没跑？）", outDir))
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, cliErrors.New(cliErrors.ONE_CLI_ERROR,
			fmt.Sprintf("构建产物应该是目录，但 %s 是文件", outDir))
	}
	var out []uploadFile
	err = filepath.Walk(outDir, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fi.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(outDir, path)
		if relErr != nil {
			return relErr
		}
		out = append(out, uploadFile{
			relPath: filepath.ToSlash(rel),
			absPath: path,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func newS3Client(ctx context.Context, ep *Endpoint, creds *Credentials) (*s3.Client, error) {
	staticCreds := credentials.NewStaticCredentialsProvider(
		creds.AccessKeyID, creds.AccessKeySecret, "")
	region := strings.TrimSpace(ep.Region)
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := awsConfig.LoadDefaultConfig(ctx,
		awsConfig.WithRegion(region),
		awsConfig.WithCredentialsProvider(staticCreds),
	)
	if err != nil {
		return nil, err
	}
	opts := []func(*s3.Options){
		func(o *s3.Options) {
			if strings.TrimSpace(ep.Endpoint) != "" {
				o.BaseEndpoint = &ep.Endpoint
			}
			// S3-compatible providers such as Aliyun OSS reject the SDK's
			// default trailer-checksum aws-chunked upload mode. Compute
			// request checksums only when an operation explicitly requires it.
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
			if ep.ForcePathStyle {
				o.UsePathStyle = true
			}
		},
	}
	return s3.NewFromConfig(cfg, opts...), nil
}

type objectClient interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

func uploadOne(ctx context.Context, client objectClient, bucket string, f uploadFile) error {
	body, err := os.Open(f.absPath)
	if err != nil {
		return err
	}
	defer body.Close()
	info, err := body.Stat()
	if err != nil {
		return err
	}
	contentType := mime.TypeByExtension(filepath.Ext(f.relPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	in := &s3.PutObjectInput{
		Bucket:        &bucket,
		Key:           &f.relPath,
		Body:          body,
		ContentLength: aws.Int64(info.Size()),
		ContentType:   &contentType,
	}
	_, err = client.PutObject(ctx, in)
	return err
}

func endpointDisplay(ep *Endpoint) string {
	if ep == nil || strings.TrimSpace(ep.Endpoint) == "" {
		return "AWS S3 (default)"
	}
	return ep.Endpoint
}

func bucketFromEndpoint(ep *Endpoint) string {
	if ep == nil {
		return "(no bucket)"
	}
	return ep.Bucket
}
