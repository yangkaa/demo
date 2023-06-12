package main

import (
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/opentracing/opentracing-go/ext"
	jaegercfg "github.com/uber/jaeger-client-go/config"
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
		jaegercfg.Logger(jaeger.StdLogger),
		//jaegercfg.Metrics(metricsFactory),
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
	spanCtx, _ := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header),
	)
	span := opentracing.GlobalTracer().StartSpan(
		"operation_name",
		ext.RPCServerOption(spanCtx),
	)
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
	clientReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	clientReq.Header.Add("X-Request-ID", req.Header.Get("X-Request-ID"))
	serviceCResp := makeRequest(clientReq, "serviceC")

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
