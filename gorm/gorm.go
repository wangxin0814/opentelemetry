package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	otgorm "github.com/smacker/opentracing-gorm"
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

var db *gorm.DB

type Product struct {
	gorm.Model
	Code string
}

func main() {
	tp, err := Init("gorm")
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = tp.Shutdown(context.Background()) }()

	r := gin.New()
	r.Use(otelgin.Middleware("trace middleware"))

	// connect postgres
	db = initDB()

	r.GET("/insert", gormInsert)

	r.GET("/query", gormQuery)

	_ = r.Run(":8080")

}
func gormInsert(c *gin.Context) {
	ctx := c.Request.Context()

	var span trace.Span
	tracer := traceFromContext(ctx)
	ctx, span = tracer.Start(ctx, "gorm insert")

	db := otgorm.SetSpanToGorm(ctx, db)
	defer span.End()

	err := db.Exec(`insert into product (code) values ($1)`, "hello").Error

	//err := db.Create(&Product{
	//	Code: "hello",
	//}).Error
	if err != nil {
		span.SetStatus(codes.Error, "failed") // Necessary
		span.RecordError(err)
	} else {
		span.AddEvent("insert")
	}
}

func gormQuery(c *gin.Context) {
	ctx := c.Request.Context()

	var span trace.Span
	tracer := traceFromContext(ctx)
	ctx, span = tracer.Start(ctx, "gorm query")

	db := otgorm.SetSpanToGorm(ctx, db)
	defer span.End()

	var product Product
	err := db.First(&product).Error
	if err != nil {
		span.SetStatus(codes.Error, "failed") // Necessary
		span.RecordError(err)
	} else {
		span.AddEvent("query", trace.WithAttributes(attribute.Int64("product.id", int64(product.ID))))
	}
}

func initDB() *gorm.DB {
	db, err := gorm.Open("postgres", "postgres://postgres:123456@192.168.0.120:5432/opentelemetry?sslmode=disable")
	if err != nil {
		panic(err)
	}
	db.DB().SetMaxOpenConns(1000)
	db.DB().SetMaxIdleConns(10)
	db.DB().SetConnMaxLifetime(time.Hour)
	for {
		if err = db.DB().Ping(); err != nil {
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}

	db.LogMode(true)
	db.SingularTable(true)

	db.AutoMigrate(&Product{})

	otgorm.AddGormCallbacks(db)

	return db
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
