package main

import (
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

func makeRequest(req *http.Request) string {
	span, _ := opentracing.StartSpanFromContext(
		req.Context(),
		"makeRequest",
	)
	defer span.Finish()
	span.LogKV("event", "make req to C")
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

var serviceName = "service-b"

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

	// Make a request to serviceC
	url := "http://localhost:8082"
	protocol := "http://"
	if os.Getenv("CUSTOM_PROTOCOL") != "" {
		protocol = os.Getenv("CUSTOM_PROTOCOL")
	}
	if os.Getenv("SERVICE_C_HOST") != "" && os.Getenv("SERVICE_C_PORT") != "" {
		url = fmt.Sprintf("%s%s:%s", protocol, os.Getenv("SERVICE_C_HOST"), os.Getenv("SERVICE_C_PORT"))
	}

	ctx := opentracing.ContextWithSpan(req.Context(), span)
	clientReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	//clientReq.Header.Add("X-Request-ID", req.Header.Get("X-Request-ID"))
	serviceCResp := makeRequest(clientReq)

	// Process the request or perform any operations
	//fmt.Fprintln(w, "Hello from serviceB!", "--->", serviceCResp)
	fmt.Fprintln(w, "serviceB: Hello from serviceB!\n")
	fmt.Fprintln(w, headers, "\n")
	fmt.Fprintln(w, w.Header())
	fmt.Fprintln(w, "\n-----------------------------------------------------------------------------------------\n")
	fmt.Fprintln(w, serviceCResp)
}

func main() {
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":8081", nil))
}
