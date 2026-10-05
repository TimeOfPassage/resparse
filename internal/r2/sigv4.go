// Package r2 提供 Cloudflare R2（S3 兼容 API）的访问能力。
//
// 对应参考项目 src/lib/server/r2.ts。参考项目依赖 @aws-sdk/client-s3，
// 这里用标准库实现 AWS Signature V4 签名，保持零第三方依赖与单二进制形态。
// R2 仅需 GET / HEAD 对象，故签名逻辑只覆盖这两类无 body 请求。
package r2

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// emptyPayloadSHA256 是空字符串的 SHA-256 十六进制表示（GET/HEAD 无 body）。
const emptyPayloadSHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

const (
	algorithm       = "AWS4-HMAC-SHA256"
	serviceName     = "s3"
	amzDateFormat   = "20060102T150405Z"
	dateStampFormat = "20060102"
)

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// uriEncode 按 AWS SigV4 规则对字符串做百分号编码。
// 非保留字符保持原样，其余（含空格、非 ASCII）编码为大写十六进制。
func uriEncode(s string, encodeSlash bool) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		case c == '/' && !encodeSlash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// signRequest 为请求添加 SigV4 签名相关头部。
//
// canonicalURI 取 req.URL.EscapedPath()，保证与最终发送的请求目标一致。
// 本服务发出的请求均无查询串，故 CanonicalQueryString 为空。
func signRequest(req *http.Request, accessKey, secretKey, region string, now time.Time) {
	now = now.UTC()
	amzDate := now.Format(amzDateFormat)
	dateStamp := now.Format(dateStampFormat)

	req.Header.Set("x-amz-content-sha256", emptyPayloadSHA256)
	req.Header.Set("x-amz-date", amzDate)

	host := req.Host
	canonicalHeaders := "host:" + host + "\n" +
		"x-amz-content-sha256:" + emptyPayloadSHA256 + "\n" +
		"x-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		"", // canonical query string
		canonicalHeaders,
		signedHeaders,
		emptyPayloadSHA256,
	}, "\n")

	credentialScope := dateStamp + "/" + region + "/" + serviceName + "/aws4_request"
	stringToSign := strings.Join([]string{
		algorithm,
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(serviceName))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	authorization := algorithm +
		" Credential=" + accessKey + "/" + credentialScope +
		", SignedHeaders=" + signedHeaders +
		", Signature=" + signature
	req.Header.Set("Authorization", authorization)
}
