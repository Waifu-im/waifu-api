// Package storage provides the S3-compatible object storage backend.
//
// # Provider compatibility
//
// The service runs against whatever S3-compatible provider the deployment uses.
// The known ones are documented below, because each has at least one quirk that
// makes the plain AWS defaults fail.
//
//   - AWS S3: the reference. Virtual-host addressing, sigv4, region set.
//   - Cloudflare R2: sigv4, but rejects the trailing checksum headers that newer
//     AWS SDKs send by default. Needs the checksum mode turned down and the
//     region set to "auto".
//   - IDrive: sigv4 with per-region endpoints. Does not allow public buckets, so
//     objects stay private and are served through the Cloudflare Worker.
//   - MinIO and other self-hosted: almost always needs path-style addressing
//     because the bucket is not resolvable as a subdomain.
//
// Rather than making operators guess, the provider is selected by name and the
// defaults for it are applied automatically. Anything not listed falls through
// to generic handling with explicit configuration.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Provider identifies an S3-compatible service.
//
// It exists so that the handful of provider-specific quirks can be handled in
// one place rather than as conditional environment variables scattered through
// the codebase.
type Provider string

// Supported providers.
const (
	// ProviderGeneric applies no provider-specific behaviour. Every setting is
	// taken from configuration.
	ProviderGeneric Provider = "generic"
	// ProviderAWS is Amazon S3.
	ProviderAWS Provider = "aws"
	// ProviderR2 is Cloudflare R2.
	ProviderR2 Provider = "r2"
	// ProviderIDrive is IDrive S3, which does not permit public buckets.
	ProviderIDrive Provider = "idrive"
	// ProviderMinIO is MinIO and similar self-hosted servers.
	ProviderMinIO Provider = "minio"
)

// Config describes the bucket to write to.
type Config struct {
	// Provider selects provider-specific defaults. Unknown values fall back to
	// generic rather than failing, so a typo does not block startup; the
	// configuration is validated separately.
	Provider Provider

	AccessKey string
	SecretKey string

	// ServiceURL is the endpoint. A bare host is accepted and given https://,
	// because some existing deployments are configured that way.
	ServiceURL string

	// Region is the provider region. R2 uses "auto".
	Region string

	BucketName string

	// ForcePathStyle addresses the bucket as a path segment rather than a
	// subdomain. Required by MinIO and anything without wildcard DNS.
	ForcePathStyle bool
}

// providerSettings holds the defaults a provider implies.
type providerSettings struct {
	// regionOverride replaces an empty configured region.
	regionOverride string

	// forcePathStyle is OR'd with the configured value.
	forcePathStyle bool

	// disableRequestChecksum stops the SDK sending checksum headers.
	//
	// Cloudflare R2 rejects requests carrying x-amz-checksum-* with an
	// InvalidRequest error, because it does not implement the header trailer
	// semantics the SDK assumes. Turning the calculation off makes the SDK omit
	// them. The object is still integrity-checked by S3's own MD5 handling.
	disableRequestChecksum bool
}

// settingsFor returns the defaults for a provider.
func settingsFor(provider Provider) providerSettings {
	switch Provider(strings.ToLower(strings.TrimSpace(string(provider)))) {
	case ProviderR2:
		return providerSettings{
			regionOverride:         "auto",
			disableRequestChecksum: true,
		}
	case ProviderMinIO:
		return providerSettings{
			forcePathStyle: true,
		}
	case ProviderIDrive:
		// IDrive has historically needed path-style on some regions.
		return providerSettings{
			forcePathStyle: true,
		}
	default:
		return providerSettings{}
	}
}

// S3Store is a Store backed by S3 or any S3-compatible service.
//
// Buckets are written private. IDrive and similar providers no longer allow
// public buckets, and this deployment already serves images through a
// Cloudflare Worker that signs requests to storage, so public ACLs are neither
// expected nor used.
type S3Store struct {
	client   *s3.Client
	bucket   string
	endpoint string
	provider Provider
}

// NewS3Store builds a store and verifies the required settings are present.
//
// Every missing value is reported at once, so a misconfigured deployment is
// fixed in one pass instead of through repeated restarts.
func NewS3Store(cfg Config) (*S3Store, error) {
	settings := settingsFor(cfg.Provider)

	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = settings.regionOverride
	}

	required := map[string]string{
		"S3__AccessKey":  cfg.AccessKey,
		"S3__SecretKey":  cfg.SecretKey,
		"S3__ServiceUrl": cfg.ServiceURL,
		"S3__Region":     region,
		"S3__BucketName": cfg.BucketName,
	}

	missing := make([]string, 0, len(required))
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		// Sorted so the message is stable between runs.
		sortStrings(missing)
		return nil, fmt.Errorf("S3 configuration is incomplete, missing: %s",
			strings.Join(missing, ", "))
	}

	ctx := context.Background()

	loadOptions := awsconfig.LoadOptions{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	}

	if settings.disableRequestChecksum {
		// See providerSettings for why. Cloudflare R2 rejects the trailing
		// x-amz-checksum-* headers that the SDK otherwise sends.
		loadOptions.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		loadOptions.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	}

	// LoadDefaultConfig takes option functions, so the assembled options are
	// applied through a single closure.
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, func(o *awsconfig.LoadOptions) error {
		*o = loadOptions
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	endpoint := normalizeEndpoint(cfg.ServiceURL)

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		if endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
		options.UsePathStyle = cfg.ForcePathStyle || settings.forcePathStyle
	})

	return &S3Store{
		client:   client,
		bucket:   cfg.BucketName,
		endpoint: endpoint,
		provider: cfg.Provider,
	}, nil
}

// normalizeEndpoint turns a configured endpoint into a base URL.
//
// A bare host gets https://, since every supported provider is reached over TLS.
// Trailing slashes are stripped because the SDK joins paths onto this value.
func normalizeEndpoint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		trimmed = "https://" + trimmed
	}
	return strings.TrimRight(trimmed, "/")
}

// Provider reports which provider this store was configured for.
func (s *S3Store) Provider() Provider { return s.provider }

// Put writes an object.
//
// The body is buffered before sending so a read failure partway through cannot
// leave a truncated object behind. A truncated object that reports success would
// be worse than a failed request, because the catalogue row would point at
// broken bytes with no indication anything is wrong.
func (s *S3Store) Put(ctx context.Context, key string, body io.Reader, contentType string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read upload body: %w", err)
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, describe(err))
	}
	return nil
}

// Delete removes an object.
//
// A missing object is not an error, because the upload rollback calls this after
// a partial failure and must be idempotent.
func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete object %s: %w", key, describe(err))
	}
	return nil
}

// Exists reports whether an object is present.
func (s *S3Store) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("head object %s: %w", key, describe(err))
	}
	return true, nil
}

// isNotFound reports whether an error means the object or bucket is absent.
//
// Providers disagree on the concrete type, so the API error code is checked as
// well: "NoSuchKey", "NotFound" and "404" all appear in the wild.
func isNotFound(err error) bool {
	var noKey *types.NoSuchKey
	if errors.As(err, &noKey) {
		return true
	}
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}

	var apiErr interface{ ErrorCode() string }
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		}
	}
	return false
}

// describe unwraps the smithy layer so the log line names the real failure
// rather than a serialization error.
func describe(err error) error {
	var apiErr interface{ ErrorMessage() string }
	if errors.As(err, &apiErr) {
		if message := apiErr.ErrorMessage(); message != "" {
			return errors.New(message)
		}
	}
	return err
}

// sortStrings is a tiny insertion sort, used so error messages are stable without
// pulling in sort for one call site.
func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		value := values[i]
		j := i - 1
		for j >= 0 && values[j] > value {
			values[j+1] = values[j]
			j--
		}
		values[j+1] = value
	}
}

// List returns every object key in the bucket.
//
// Paginated in batches, because a bucket with thousands of images returns a
// truncated page rather than an error. Continuation is followed until exhausted,
// and a mid-walk failure returns what was collected along with the error so a
// partial reconciliation is possible rather than none.
func (s *S3Store) List(ctx context.Context) ([]string, error) {
	const pageSize = 1000

	var keys []string
	var token *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			MaxKeys:           aws.Int32(pageSize),
			ContinuationToken: token,
		}

		output, err := s.client.ListObjectsV2(ctx, input)
		if err != nil {
			return keys, fmt.Errorf("list objects: %w", describe(err))
		}

		for _, object := range output.Contents {
			if object.Key != nil {
				keys = append(keys, *object.Key)
			}
		}

		if output.IsTruncated == nil || !*output.IsTruncated || output.NextContinuationToken == nil {
			break
		}
		token = output.NextContinuationToken
	}

	return keys, nil
}
