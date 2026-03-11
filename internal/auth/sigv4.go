// Package auth implements AWS Signature Version 4 request verification.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	iso8601Format = "20060102T150405Z"
	dateFormat    = "20060102"
	awsAlgorithm  = "AWS4-HMAC-SHA256"
)

// Verifier checks AWS Signature V4 on incoming requests.
type Verifier struct {
	accessKey string
	secretKey string
	region    string
}

// NewVerifier creates a new Verifier.
func NewVerifier(accessKey, secretKey, region string) *Verifier {
	return &Verifier{
		accessKey: accessKey,
		secretKey: secretKey,
		region:    region,
	}
}

// Verify returns true if the request carries a valid AWS Signature V4 or
// valid presigned URL parameters. It also returns true if no auth header is
// present at all (handled by the caller per-route policy).
func (v *Verifier) Verify(r *http.Request) error {
	// Handle presigned URLs (query-string auth).
	if r.URL.Query().Get("X-Amz-Signature") != "" {
		return v.verifyPresigned(r)
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("missing Authorization header")
	}

	return v.verifyHeader(r, authHeader)
}

// verifyHeader validates standard Authorization header signing.
func (v *Verifier) verifyHeader(r *http.Request, authHeader string) error {
	// Parse: AWS4-HMAC-SHA256 Credential=.../..., SignedHeaders=..., Signature=...
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != awsAlgorithm {
		return fmt.Errorf("unsupported authorization algorithm")
	}

	fields := parseCSV(parts[1])
	credential := fields["Credential"]
	signedHeaders := fields["SignedHeaders"]
	providedSig := fields["Signature"]

	credParts := strings.Split(credential, "/")
	if len(credParts) < 5 {
		return fmt.Errorf("malformed Credential")
	}
	if credParts[0] != v.accessKey {
		return fmt.Errorf("unknown access key")
	}
	dateStamp := credParts[1]
	region := credParts[2]
	service := credParts[3]

	// Reconstruct the date/time from X-Amz-Date header.
	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		return fmt.Errorf("missing X-Amz-Date header")
	}
	t, err := time.Parse(iso8601Format, amzDate)
	if err != nil {
		return fmt.Errorf("invalid X-Amz-Date: %w", err)
	}

	// Build canonical request.
	bodyHash := r.Header.Get("X-Amz-Content-Sha256")
	if bodyHash == "" {
		bodyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	}

	canonicalRequest := buildCanonicalRequest(r, signedHeaders, bodyHash)
	expectedSig := v.computeSignature(canonicalRequest, dateStamp, region, service, t)

	if !hmac.Equal([]byte(providedSig), []byte(expectedSig)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// verifyPresigned validates presigned URL query-string auth.
func (v *Verifier) verifyPresigned(r *http.Request) error {
	q := r.URL.Query()

	algo := q.Get("X-Amz-Algorithm")
	if algo != awsAlgorithm {
		return fmt.Errorf("unsupported algorithm: %s", algo)
	}

	credential := q.Get("X-Amz-Credential")
	credParts := strings.Split(credential, "/")
	if len(credParts) < 5 {
		return fmt.Errorf("malformed credential")
	}
	if credParts[0] != v.accessKey {
		return fmt.Errorf("unknown access key")
	}
	dateStamp := credParts[1]
	region := credParts[2]
	service := credParts[3]

	amzDate := q.Get("X-Amz-Date")
	t, err := time.Parse(iso8601Format, amzDate)
	if err != nil {
		return fmt.Errorf("invalid X-Amz-Date: %w", err)
	}

	// Check expiry.
	expiresStr := q.Get("X-Amz-Expires")
	if expiresStr != "" {
		var expires int
		fmt.Sscanf(expiresStr, "%d", &expires)
		if time.Since(t) > time.Duration(expires)*time.Second {
			return fmt.Errorf("presigned URL has expired")
		}
	}

	signedHeaders := q.Get("X-Amz-SignedHeaders")
	providedSig := q.Get("X-Amz-Signature")

	// For presigned URLs, the payload hash is UNSIGNED-PAYLOAD.
	canonicalRequest := buildCanonicalRequestPresigned(r, signedHeaders, q)
	expectedSig := v.computeSignature(canonicalRequest, dateStamp, region, service, t)

	if !hmac.Equal([]byte(providedSig), []byte(expectedSig)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// computeSignature runs through the AWS4 signing steps.
func (v *Verifier) computeSignature(canonicalRequest, dateStamp, region, service string, t time.Time) string {
	scope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, service)
	stringToSign := strings.Join([]string{
		awsAlgorithm,
		t.UTC().Format(iso8601Format),
		scope,
		hashSHA256(canonicalRequest),
	}, "\n")

	signingKey := deriveSigningKey(v.secretKey, dateStamp, region, service)
	return hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
}

// buildCanonicalRequest constructs the canonical request string for header-auth.
func buildCanonicalRequest(r *http.Request, signedHeaders, bodyHash string) string {
	method := r.Method
	uri := r.URL.EscapedPath()
	if uri == "" {
		uri = "/"
	}

	query := buildCanonicalQueryString(r)
	headers := buildCanonicalHeaders(r, signedHeaders)

	return strings.Join([]string{method, uri, query, headers, signedHeaders, bodyHash}, "\n")
}

// buildCanonicalRequestPresigned constructs the canonical request for presigned URLs.
func buildCanonicalRequestPresigned(r *http.Request, signedHeaders string, q interface{ Get(string) string }) string {
	method := r.Method
	uri := r.URL.EscapedPath()
	if uri == "" {
		uri = "/"
	}
	query := buildCanonicalQueryStringPresigned(r)
	headers := buildCanonicalHeaders(r, signedHeaders)

	return strings.Join([]string{method, uri, query, headers, signedHeaders, "UNSIGNED-PAYLOAD"}, "\n")
}

func buildCanonicalQueryString(r *http.Request) string {
	params := r.URL.Query()
	// Remove signature param for header auth (shouldn't be there, but be safe).
	delete(params, "X-Amz-Signature")

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		vals := params[k]
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, uriEncode(k)+"="+uriEncode(v))
		}
	}
	return strings.Join(parts, "&")
}

func buildCanonicalQueryStringPresigned(r *http.Request) string {
	params := r.URL.Query()
	// Remove the signature itself — it's not part of the signed query.
	delete(params, "X-Amz-Signature")

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		vals := params[k]
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, uriEncode(k)+"="+uriEncode(v))
		}
	}
	return strings.Join(parts, "&")
}

func buildCanonicalHeaders(r *http.Request, signedHeaders string) string {
	headers := strings.Split(signedHeaders, ";")
	var sb strings.Builder
	for _, h := range headers {
		val := ""
		if h == "host" {
			val = r.Host
			if val == "" {
				val = r.Header.Get("Host")
			}
		} else {
			val = strings.TrimSpace(r.Header.Get(h))
		}
		sb.WriteString(strings.ToLower(h))
		sb.WriteString(":")
		sb.WriteString(val)
		sb.WriteString("\n")
	}
	return sb.String()
}

// deriveSigningKey produces the AWS4 signing key via HMAC chains.
func deriveSigningKey(secretKey, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func hashSHA256(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// parseCSV parses "Key=Value, Key=Value, ..." into a map.
func parseCSV(s string) map[string]string {
	result := make(map[string]string)
	for _, part := range strings.Split(s, ", ") {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return result
}

// uriEncode encodes a string per AWS URI encoding rules.
func uriEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUnreserved(c) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func isUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.' || c == '~'
}
