package main

import (
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go/transport"
	"github.com/uber/jaeger-client-go/zipkin"
)

var TracingAnalysisEndpoint = "http://47.104.161.96:14268/api/traces"

func init() {
	if os.Getenv("OTEL_EXPORTER_JAEGER_ENDPOINT") != "" {
		TracingAnalysisEndpoint = os.Getenv("OTEL_EXPORTER_JAEGER_ENDPOINT")
	}
	log.Printf("TracingAnalysisEndpoint is : %v", TracingAnalysisEndpoint)
}

func initTracer(serviceName string) (opentracing.Tracer, io.Closer) {
	zipkinPropagator := zipkin.NewZipkinB3HTTPHeaderPropagator()
	injector := jaeger.TracerOptions.Injector(opentracing.HTTPHeaders, zipkinPropagator)
	extractor := jaeger.TracerOptions.Extractor(opentracing.HTTPHeaders, zipkinPropagator)
	// Zipkin shares span ID between client and server spans; it must be enabled via the following option.
	zipkinSharedRPCSpan := jaeger.TracerOptions.ZipkinSharedRPCSpan(true)

	tracer, closer := jaeger.NewTracer(
		serviceName,
		jaeger.NewConstSampler(true),
		jaeger.NewRemoteReporter(
			transport.NewHTTPTransport(TracingAnalysisEndpoint),
			jaeger.ReporterOptions.Logger(jaeger.StdLogger),
		),
		injector,
		extractor,
		zipkinSharedRPCSpan,
	)
	opentracing.SetGlobalTracer(tracer)
	return tracer, closer
}

var serviceName = "service-c"

func handler(w http.ResponseWriter, req *http.Request) {
	// Get All Headers
	var headers string
	for key, values := range req.Header {
		if key == "Cookie" {
			continue
		}
		for _, v := range values {
			headers += fmt.Sprintf("%s:   %v\n", key, v)
		}
	}
	fmt.Println(headers)
	tracer, closer := initTracer(serviceName)
	defer closer.Close()

	spanCtx, _ := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
	span := tracer.StartSpan("handler", ext.RPCServerOption(spanCtx))
	defer span.Finish()

	// Process the request or perform any operations
	fmt.Fprintln(w, "serviceC: Hello from serviceC!\n")
	fmt.Fprintln(w, headers, "\n")
	fmt.Fprintln(w, w.Header())
}

func main() {
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":8082", nil))
}
