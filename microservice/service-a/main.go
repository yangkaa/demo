package main

import (
	"context"
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport"
	"github.com/uber/jaeger-client-go/zipkin"
	"io"
	"log"
	"net/http"
	"os"
)

const TracingAnalysisEndpoint = "http://47.104.161.96:14268/api/traces"

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

func makeRequest(req *http.Request) string {
	span, _ := opentracing.StartSpanFromContext(
		req.Context(),
		"makeRequest",
	)
	defer span.Finish()
	span.LogKV("event", "make req to B")
	ext.SpanKindRPCClient.Set(span)
	ext.HTTPUrl.Set(span, req.URL.String())
	ext.HTTPMethod.Set(span, "GET")
	span.Tracer().Inject(
		span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header),
	)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		ext.LogError(span, err)
		log.Printf("Request failed: %v", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		return ""
	}

	return fmt.Sprintf("%s\n", string(body))
}

var serviceName = "service-a"

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

	tracer, closer := initTracer(serviceName)
	defer closer.Close()

	spanCtx, _ := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
	span := tracer.StartSpan("handler", ext.RPCServerOption(spanCtx))
	defer span.Finish()
	ctx := context.Background()
	ctx = opentracing.ContextWithSpan(ctx, span)

	// Make a request to serviceB
	url := "http://localhost:8081"
	protocol := "http://"
	if os.Getenv("CUSTOM_PROTOCOL") != "" {
		protocol = os.Getenv("CUSTOM_PROTOCOL")
	}
	if os.Getenv("SERVICE_B_HOST") != "" && os.Getenv("SERVICE_B_PORT") != "" {
		url = fmt.Sprintf("%s%s:%s", protocol, os.Getenv("SERVICE_B_HOST"), os.Getenv("SERVICE_B_PORT"))
	}
	clientReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	//clientReq.Header.Add("X-Request-ID", req.Header.Get("X-Request-ID"))
	serviceBResp := makeRequest(clientReq)

	// Continue processing the request or send response to the client

	fmt.Fprintln(w, "serviceA: Hello from serviceA!\n")
	fmt.Fprintln(w, headers, "\n")
	fmt.Fprintln(w, "\n-----------------------------------------------------------------------------------------\n")
	fmt.Fprintln(w, serviceBResp)
}

func main() {
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
