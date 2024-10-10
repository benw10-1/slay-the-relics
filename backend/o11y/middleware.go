package o11y

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// nil checks for Tracer and Meter are included for api handler tests (for now)
func Middleware(c *gin.Context) {
	var err error
	ctx := c.Request.Context()
	var span trace.Span
	if Tracer != nil {
		ctx, span = Tracer.Start(c.Request.Context(), "http.request", trace.WithSpanKind(trace.SpanKindServer))
		defer End(&span, &err)
	}

	req := c.Request.WithContext(ctx)
	c.Request = req

	target := req.URL.Path
	method := req.Method
	contentLength := req.ContentLength

	if span != nil {
		span.SetAttributes(
			attribute.String("http.url", req.URL.String()),
			attribute.String("http.target", target),
			attribute.String("http.method", method),
			attribute.String("http.user_agent", req.UserAgent()),
			attribute.String("http.host", req.Host),
			attribute.String("http.scheme", req.URL.Scheme),
			attribute.Int64("http.request_content_length", contentLength),
		)
	}

	c.Next()

	err = c.Err()
	status := c.Writer.Status()

	if Meter != nil {
		requestCounter, _ := Meter.Int64Counter("http.requests")
		requestHistogram, _ := Meter.Int64Histogram("http.requests.content_length")
		if requestCounter != nil {
			requestCounter.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("target", target),
					attribute.String("method", method),
					attribute.Int("status_code", status),
				),
			)
		}
		if requestHistogram != nil {
			requestHistogram.Record(ctx, contentLength,
				metric.WithAttributes(
					attribute.String("target", target),
					attribute.String("method", method),
					attribute.Int("status_code", status),
				),
			)
		}
	}

	if span != nil {
		span.SetAttributes(attribute.Int("http.status_code", status))
	}
}
