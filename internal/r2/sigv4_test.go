package r2

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestURIEncode(t *testing.T) {
	cases := []struct {
		in          string
		encodeSlash bool
		want        string
	}{
		{"videos/demo.mp4", false, "videos/demo.mp4"},
		{"a b/c", false, "a%20b/c"},
		{"a/b", true, "a%2Fb"},
		{"~-_.", false, "~-_."},
		{"中文.mp4", false, "%E4%B8%AD%E6%96%87.mp4"},
		{"a+b=c&d", false, "a%2Bb%3Dc%26d"},
	}
	for _, c := range cases {
		if got := uriEncode(c.in, c.encodeSlash); got != c.want {
			t.Errorf("uriEncode(%q, %v) = %q, want %q", c.in, c.encodeSlash, got, c.want)
		}
	}
}

// TestSigV4KnownVector 用 AWS 官方 get-vanilla 测试向量校验签名原语。
// 参考：AWS Signature Version 4 签名过程文档中的示例。
func TestSigV4KnownVector(t *testing.T) {
	secret := "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
	dateStamp := "20150830"
	amzDate := "20150830T123600Z"
	region := "us-east-1"
	service := "service"

	canonicalRequest := strings.Join([]string{
		"GET",
		"/",
		"",
		"host:example.amazonaws.com\nx-amz-date:" + amzDate + "\n",
		"host;x-amz-date",
		emptyPayloadSHA256,
	}, "\n")

	credentialScope := dateStamp + "/" + region + "/" + service + "/aws4_request"
	stringToSign := strings.Join([]string{
		algorithm,
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	const want = "5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31"
	if signature != want {
		t.Fatalf("signature = %s, want %s", signature, want)
	}
}
