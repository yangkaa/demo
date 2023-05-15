package main

import (
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func initTracer(serviceName string) (opentracing.Tracer, error) {
	//metricsFactory := prometheus.New()
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
		//jaegercfg.Metrics(metricsFactory),
		jaegercfg.Logger(jaeger.StdLogger),
	)
	if err != nil {
		log.Printf("Failed to create tracer: %v", err)
		return nil, err
	}
	return tracer, nil
}

func makeRequest(req *http.Request, serviceName string) string {
	tracer, err := initTracer(serviceName)
	if err != nil {
		log.Fatal(err)
	}
	span := tracer.StartSpan("operation_name")
	defer span.Finish()
	opentracing.GlobalTracer().Inject(
		span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header),
	)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Request failed: %v", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		return ""
	}

	return fmt.Sprintf("%s\n", string(body))
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
	//fmt.Println(headers)
	spanCtx, _ := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header),
	)
	span := opentracing.GlobalTracer().StartSpan(
		"operation_name",
		ext.RPCServerOption(spanCtx),
	)
	defer span.Finish()

	// Make a request to serviceB
	url := "http://localhost:8081"
	protocol := "http://"
	if os.Getenv("PROTOCOL") != "" {
		protocol = os.Getenv("PROTOCOL")
	}
	if os.Getenv("SERVICE_B_HOST") != "" && os.Getenv("SERVICE_B_PORT") != "" {
		url = fmt.Sprintf("%s%s:%s", protocol, os.Getenv("SERVICE_B_HOST"), os.Getenv("SERVICE_B_PORT"))
	}
	clientReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	serviceBResp := makeRequest(clientReq, "serviceB")

	// Continue processing the request or send response to the client

	fmt.Fprintln(w, "serviceA: Hello from serviceA!\n")
	fmt.Fprintln(w, headers)
	fmt.Fprintln(w, "\n-----------------------------------------------------------------------------------------\n")
	fmt.Fprintln(w, serviceBResp)
}

func main() {
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
