package r2

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"resparse/internal/apperr"
	"resparse/internal/config"
)

// Head 描述对象的元信息（不含内容）。缺失字段为 nil，序列化时表现为 null。
type Head struct {
	SizeBytes    *int64
	ContentType  *string
	ETag         *string
	LastModified *string
}

// Client 是 R2（S3 兼容）访问客户端。
type Client struct {
	endpoint  *url.URL
	region    string
	bucket    string
	accessKey string
	secretKey string
	http      *http.Client
	now       func() time.Time
}

// NewClient 由解析后的 R2 配置构造客户端。
func NewClient(cfg config.R2Config) (*Client, error) {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, apperr.New("config_missing", "R2 endpoint 非法："+cfg.Endpoint, 500, nil)
	}
	return &Client{
		endpoint:  u,
		region:    cfg.Region,
		bucket:    cfg.Bucket,
		accessKey: cfg.AccessKeyID,
		secretKey: cfg.SecretAccessKey,
		http:      &http.Client{},
		now:       time.Now,
	}, nil
}

// Bucket 返回当前 bucket 名称。
func (c *Client) Bucket() string { return c.bucket }

// newRequest 构造指向对象的请求（path-style：/<bucket>/<encoded-key>）。
//
// 通过同时设置 Path 与 RawPath，使 req.URL.EscapedPath() 返回我们自己按
// SigV4 规则编码的路径，从而保证实际请求目标与签名中的 CanonicalURI 一致。
func (c *Client) newRequest(ctx context.Context, method, key string) (*http.Request, error) {
	rawPath := "/" + c.bucket + "/" + uriEncode(key, false)
	decoded, err := url.PathUnescape(rawPath)
	if err != nil {
		decoded = rawPath
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://placeholder", nil)
	if err != nil {
		return nil, err
	}
	req.URL = &url.URL{
		Scheme:  c.endpoint.Scheme,
		Host:    c.endpoint.Host,
		Path:    decoded,
		RawPath: rawPath,
	}
	req.Host = c.endpoint.Host
	return req, nil
}

// Head 读取对象元信息（不含内容）。
func (c *Client) Head(ctx context.Context, key string) (Head, error) {
	req, err := c.newRequest(ctx, http.MethodHead, key)
	if err != nil {
		return Head{}, r2Error(err)
	}
	signRequest(req, c.accessKey, c.secretKey, c.region, c.now())

	resp, err := c.http.Do(req)
	if err != nil {
		return Head{}, r2Error(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if err := c.checkStatus(resp, key); err != nil {
		return Head{}, err
	}
	return headFromResponse(resp), nil
}

// Get 打开对象并返回可读流及元信息。
//
// 若调用方已 HEAD 过（head 非 nil），其字段会覆盖从 GET 响应推导出的值。
func (c *Client) Get(ctx context.Context, key string, head *Head) (io.ReadCloser, Head, error) {
	req, err := c.newRequest(ctx, http.MethodGet, key)
	if err != nil {
		return nil, Head{}, r2Error(err)
	}
	signRequest(req, c.accessKey, c.secretKey, c.region, c.now())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, Head{}, r2Error(err)
	}
	if err := c.checkStatus(resp, key); err != nil {
		resp.Body.Close()
		return nil, Head{}, err
	}

	h := headFromResponse(resp)
	if head != nil {
		if head.SizeBytes != nil {
			h.SizeBytes = head.SizeBytes
		}
		if head.ContentType != nil {
			h.ContentType = head.ContentType
		}
		if head.ETag != nil {
			h.ETag = head.ETag
		}
		if head.LastModified != nil {
			h.LastModified = head.LastModified
		}
	}
	return resp.Body, h, nil
}

func (c *Client) checkStatus(resp *http.Response, key string) error {
	if resp.StatusCode == http.StatusNotFound {
		return apperr.New("object_not_found", "R2 中不存在对象："+key, 404, nil)
	}
	if resp.StatusCode >= 300 {
		return apperr.New(
			"r2_error",
			fmt.Sprintf("访问 R2 失败：HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode)),
			502, nil,
		)
	}
	return nil
}

func headFromResponse(resp *http.Response) Head {
	var h Head
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil {
			h.SizeBytes = &n
		}
	} else if resp.ContentLength >= 0 {
		n := resp.ContentLength
		h.SizeBytes = &n
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		h.ContentType = &ct
	}
	if et := resp.Header.Get("ETag"); et != "" {
		h.ETag = &et
	}
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		if t, err := http.ParseTime(lm); err == nil {
			s := t.UTC().Format("2006-01-02T15:04:05.000Z")
			h.LastModified = &s
		} else {
			h.LastModified = &lm
		}
	}
	return h
}

func r2Error(err error) error {
	return apperr.New("r2_error", "访问 R2 失败："+err.Error(), 502, nil)
}
