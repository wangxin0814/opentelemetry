package main

import (
	"context"
	"errors"
	"fmt"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"io"
	"log"
	"net/http"
	"time"
)

//var tracer = otel.Tracer("github.com/wangxin/example")

func traceFromContext(ctx context.Context) (tracer trace.Tracer) {
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		tracer = span.TracerProvider().Tracer("github.com/wangxin/example")
	} else {
		tracer = otel.Tracer("github.com/wangxin/example")
	}

	return
}

func main() {
	//l := log.New(os.Stdout, "", 0)
	//
	//f, err := os.Create("traces.txt")
	//if err != nil {
	//	l.Fatal(err)
	//}
	//defer f.Close()
	//
	//exporter, err := newExporter(f)
	//if err != nil {
	//	return
	//}

	exporter, err := newJaegerExporter()
	if err != nil {
		return
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		//sdktrace.WithResource(newResource()),
		//sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(resource.NewSchemaless(semconv.ServiceNameKey.String("http"))),
	)

	defer tp.Shutdown(context.Background())

	//defer func() {
	//	if err := tp.Shutdown(context.Background()); err != nil {
	//		l.Fatal(err)
	//	}
	//}()

	otel.SetTracerProvider(tp)

	handler := http.HandlerFunc(httpHandler)
	wrappedHandler := otelhttp.NewHandler(handler, "hello-instrumented")

	http.Handle("/hello-instrumented", wrappedHandler)

	log.Fatal(http.ListenAndServe(":3030", nil))
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, World! I am instrumented automatically!")
	ctx := r.Context()
	sleepy(ctx)
}

func sleepy(ctx context.Context) {
	tracer := traceFromContext(ctx)
	var span trace.Span
	ctx, span = tracer.Start(ctx, "sleep")
	defer span.End()

	log.Printf("trace_id: %s, span_id: %s", span.SpanContext().TraceID().String(), span.SpanContext().SpanID().String())

	span.SetAttributes(attribute.String("hello", "world")) //

	span.AddEvent("1111")
	span.AddEvent("2222")
	span.AddEvent("3333")

	span.AddEvent("aaaa", trace.WithAttributes(attribute.Int("id", 100), attribute.String("nihao", "haha")))

	sleepTime := 1 * time.Second
	time.Sleep(sleepTime)
	span.SetAttributes(attribute.Int("sleep.duration", int(sleepTime)))

	sleepy2(ctx)
}

func sleepy2(ctx context.Context) {
	tracer := traceFromContext(ctx)
	_, span := tracer.Start(ctx, "sleep2")
	defer span.End()

	span.SetStatus(codes.Error, "failed") // Necessary
	span.RecordError(errors.New("failed"))
}

func newExporter(w io.Writer) (sdktrace.SpanExporter, error) {
	return stdouttrace.New(
		stdouttrace.WithWriter(w),
		// Use human-readable output.
		stdouttrace.WithPrettyPrint(),
		// Do not print timestamps for the demo.
		stdouttrace.WithoutTimestamps(),
	)
}

func newJaegerExporter() (sdktrace.SpanExporter, error) {
	return jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://192.168.0.120:14268/api/traces")))
}

//func newResource() *resource.Resource {
//	r, _ := resource.Merge(
//		resource.Default(),
//		resource.NewWithAttributes(
//			semconv.SchemaURL,
//			semconv.ServiceName("fib"),
//			semconv.ServiceVersion("v0.1.0"),
//			attribute.String("environment", "demo"),
//		),
//	)
//	return r
//}
