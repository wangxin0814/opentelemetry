package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	otredis "github.com/smacker/opentracing-go-redis"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"log"
	"time"
)

var client *redis.Client // initialized at program startup

func main() {
	tp, err := Init("redis")
	if err != nil {
		log.Fatal(err)
	}
	//defer func() {
	//	if err := tp.Shutdown(context.Background()); err != nil {
	//		log.Printf("Error shutting down tracer provider: %v", err)
	//	}
	//}()

	defer func() { _ = tp.Shutdown(context.Background()) }()

	r := gin.New()
	r.Use(otelgin.Middleware("trace middleware"))

	// connect redis
	client = redis.NewClient(&redis.Options{
		Addr: "192.168.0.120:6379",
	})

	defer client.Close()

	r.GET("/get", redisGet)

	r.GET("/set", redisSet)

	_ = r.Run(":8080")
}

func redisGet(c *gin.Context) {
	ctx := c.Request.Context()

	var span trace.Span
	tracer := traceFromContext(ctx)
	ctx, span = tracer.Start(ctx, "redis get")

	client = otredis.WrapRedisClient(ctx, client)
	defer span.End()

	//log.Printf("trace_id: %s, span_id: %s", span.SpanContext().TraceID().String(), span.SpanContext().SpanID().String())

	result, err := client.Get("hello").Result()

	if err != nil { // 操作错误
		span.SetStatus(codes.Error, "failed") // Necessary
		span.RecordError(err)
	} else {
		if err == redis.Nil { // 没有值
			span.AddEvent("get", trace.WithAttributes(attribute.String("key", "hello")))
		} else {
			span.AddEvent("get", trace.WithAttributes(attribute.String("key", "hello"), attribute.String("value", result)))
		}
	}
}

func redisSet(c *gin.Context) {
	ctx := c.Request.Context()

	var span trace.Span
	tracer := traceFromContext(ctx)
	ctx, span = tracer.Start(ctx, "redis set")

	client = otredis.WrapRedisClient(ctx, client)

	defer span.End()

	span.AddEvent("set", trace.WithAttributes(attribute.String("key", "hello"), attribute.String("value", "world")))

	client.Set("hello", "world", 10*time.Second)

	//span, ctx := opentracing.StartSpanFromContext(ctx, "test-set")
}

// Init creates a new instance of Jaeger tracer.
func Init(serviceName string) (*sdktrace.TracerProvider, error) {

	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://192.168.0.120:14268/api/traces")))

	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(resource.NewSchemaless(semconv.ServiceNameKey.String(serviceName))),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}))
	return tp, nil
}

func traceFromContext(ctx context.Context) (tracer trace.Tracer) {
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		tracer = span.TracerProvider().Tracer("")
	} else {
		tracer = otel.Tracer("")
	}

	return
}
