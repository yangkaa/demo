package main

import (
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"log"
	"net/http"

	"github.com/opentracing/opentracing-go/ext"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-lib/metrics/prometheus"
)

func initTracer(serviceName string) (opentracing.Tracer, error) {
	metricsFactory := prometheus.New()
	cfg := jaegercfg.Configuration{
		ServiceName: serviceName,
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans:           true,
			LocalAgentHostPort: "localhost:6831",
		},
	}
	tracer, _, err := cfg.NewTracer(
		jaegercfg.Metrics(metricsFactory),
	)
	if err != nil {
		log.Printf("Failed to create tracer: %v", err)
		return nil, err
	}
	return tracer, nil
}

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
	spanCtx, _ := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header),
	)
	span := opentracing.GlobalTracer().StartSpan(
		"operation_name",
		ext.RPCServerOption(spanCtx),
	)
	defer span.Finish()

	// Process the request or perform any operations
	fmt.Fprintln(w, "serviceC: Hello from serviceC!\n")
	fmt.Fprintln(w, headers)
}

func main() {
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":8082", nil))
}
