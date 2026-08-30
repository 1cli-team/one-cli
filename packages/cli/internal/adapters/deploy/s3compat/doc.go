// Package s3compat implements the six S3-protocol-compatible deploy
// backends (deploy/aliyun-oss, deploy/tencent-cos, deploy/aws-s3,
// deploy/minio, deploy/rustfs, deploy/r2). They all share the same
// upload implementation here — only the user-facing id, default
// endpoint/region prompts, and forcePathStyle defaults differ, and
// those are surfaced by configurecmd, not by this package. provider.go
// registers one deploy.Provider per kind so the deploy dispatcher can
// route on the manifest's bare backend id.
package s3compat
