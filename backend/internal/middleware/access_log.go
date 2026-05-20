package middleware

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	maxLoggedRequestBodySize  = 8 << 10
	maxLoggedResponseBodySize = 8 << 10
)

type bodyCaptureWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *bodyCaptureWriter) Write(data []byte) (int, error) {
	w.capture(data)
	return w.ResponseWriter.Write(data)
}

func (w *bodyCaptureWriter) WriteString(s string) (int, error) {
	w.capture([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

func (w *bodyCaptureWriter) capture(data []byte) {
	if len(data) == 0 || w.body.Len() >= maxLoggedResponseBodySize {
		return
	}
	remain := maxLoggedResponseBodySize - w.body.Len()
	if len(data) > remain {
		data = data[:remain]
	}
	_, _ = w.body.Write(data)
}

func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID, _ := c.Get("request_id")
		requestBody := summarizeRequestBody(c.Request)

		log.Printf("[HTTP] started request_id=%v method=%s path=%s query=%q ip=%s content_type=%q body=%s",
			requestID,
			c.Request.Method,
			c.Request.URL.Path,
			c.Request.URL.RawQuery,
			c.ClientIP(),
			c.GetHeader("Content-Type"),
			requestBody,
		)

		writer := &bodyCaptureWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()

		log.Printf("[HTTP] completed request_id=%v method=%s path=%s status=%d duration=%s response=%s",
			requestID,
			c.Request.Method,
			c.Request.URL.Path,
			writer.Status(),
			time.Since(start).Truncate(time.Millisecond),
			summarizeResponseBody(writer.body.Bytes(), writer.Header().Get("Content-Type")),
		)
	}
}

func summarizeRequestBody(r *http.Request) string {
	if r == nil || r.Body == nil {
		return "<empty>"
	}
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if contentType == "" {
		contentType = "unknown"
	}
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return fmt.Sprintf("<multipart body omitted size=%d>", r.ContentLength)
	}
	if !shouldLogBody(contentType) {
		return fmt.Sprintf("<body omitted content_type=%s size=%d>", contentType, r.ContentLength)
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		r.Body = io.NopCloser(bytes.NewReader(nil))
		return fmt.Sprintf("<read body error: %v>", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	if len(data) == 0 {
		return "<empty>"
	}
	truncated := len(data) > maxLoggedRequestBodySize
	if truncated {
		data = data[:maxLoggedRequestBodySize]
	}
	return formatBodyForLog(data, truncated)
}

func summarizeResponseBody(data []byte, contentType string) string {
	if len(data) == 0 {
		return "<empty>"
	}
	if !shouldLogBody(strings.ToLower(strings.TrimSpace(contentType))) {
		return fmt.Sprintf("<body omitted content_type=%s size=%d>", contentType, len(data))
	}
	return formatBodyForLog(data, len(data) >= maxLoggedResponseBodySize)
}

func shouldLogBody(contentType string) bool {
	if contentType == "" {
		return true
	}
	switch {
	case strings.Contains(contentType, "application/json"):
		return true
	case strings.Contains(contentType, "application/x-www-form-urlencoded"):
		return true
	case strings.Contains(contentType, "text/plain"):
		return true
	case strings.Contains(contentType, "text/html"):
		return true
	default:
		return false
	}
}

func formatBodyForLog(data []byte, truncated bool) string {
	body := strings.TrimSpace(string(data))
	body = strings.ReplaceAll(body, "\n", "")
	body = strings.ReplaceAll(body, "\r", "")
	body = strings.ReplaceAll(body, "\t", " ")
	if body == "" {
		body = "<empty>"
	}
	if truncated {
		return body + "...<truncated>"
	}
	return body
}
