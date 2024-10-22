package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/bojand/ghz/printer"
	"github.com/bojand/ghz/runner"
	extProcPb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/jhump/protoreflect/desc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/proto"
	klog "k8s.io/klog/v2"

	"ext-proc/backend"
	"ext-proc/handlers"
	"ext-proc/scheduling"
)

var (
	svrAddr         = flag.String("server_address", "localhost:9002", "Address of the ext proc server")
	totalRequests   = flag.Int("total_requests", 100000, "number of requests to be sent for load test")
	targetPodHeader = flag.String("targetPodHeader", "target-pod", "the header key for the target pod address to instruct Envoy to send the request to. This must match Envoy configuration.")

	// Flags when running a local ext proc server.
	numFakePods            = flag.Int("num_fake_pods", 200, "number of fake pods when running a local ext proc server")
	numModelsPerPod        = flag.Int("num_models_per_pod", 5, "number of fake models per pod when running a local ext proc server")
	localServer            = flag.Bool("local_server", true, "whether to start a local ext proc server")
	refreshPodsInterval    = flag.Duration("refreshPodsInterval", 10*time.Second, "interval to refresh pods")
	refreshMetricsInterval = flag.Duration("refreshMetricsInterval", 50*time.Millisecond, "interval to refresh metrics")
)

const (
	port = 9002
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	if *localServer {
		go startExtProc()
		time.Sleep(time.Second) // wait until server is up
		klog.Info("Server started")
	}

	report, err := runner.Run(
		"envoy.service.ext_proc.v3.ExternalProcessor.Process",
		*svrAddr,
		runner.WithInsecure(true),
		runner.WithBinaryDataFunc(generateRequest),
		runner.WithTotalRequests(uint(*totalRequests)),
	)
	if err != nil {
		klog.Fatal(err)
	}

	printer := printer.ReportPrinter{
		Out:    os.Stdout,
		Report: report,
	}

	printer.Print("summary")
}

func generateRequest(mtd *desc.MethodDescriptor, callData *runner.CallData) []byte {
	numModels := *numFakePods * (*numModelsPerPod)
	j := map[string]interface{}{
		"model":       modelName(int(callData.RequestNumber) % numModels),
		"prompt":      "Write as if you were a critic: San Francisco",
		"max_tokens":  100,
		"temperature": 0,
	}

	llmReq, err := json.Marshal(j)
	if err != nil {
		klog.Fatal(err)
	}
	req := &extProcPb.ProcessingRequest{
		Request: &extProcPb.ProcessingRequest_RequestBody{
			RequestBody: &extProcPb.HttpBody{Body: llmReq},
		},
	}
	data, err := proto.Marshal(req)
	if err != nil {
		klog.Fatal("marshaling error: ", err)
	}
	return data
}

// startExtProc starts an extProc server with fake pods.
func startExtProc() {
	pods, fm := fakePods()
	pmc := &backend.FakePodMetricsClient{Res: fm}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		klog.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	pp := backend.NewProvider(pmc, &backend.FakePodLister{Pods: pods})
	if err := pp.Init(*refreshPodsInterval, *refreshMetricsInterval); err != nil {
		klog.Fatalf("failed to initialize: %v", err)
	}
	extProcPb.RegisterExternalProcessorServer(s, handlers.NewServer(pp, scheduling.NewScheduler(pp), *targetPodHeader))

	klog.Infof("Starting gRPC server on port :%v", port)
	reflection.Register(s)
	s.Serve(lis)
}

func fakePods() (backend.PodSet, map[backend.Pod]*backend.PodMetrics) {
	pods := make(backend.PodSet)
	metrics := make(map[backend.Pod]*backend.PodMetrics, *numFakePods)
	for i := 0; i < *numFakePods; i++ {
		address := fmt.Sprintf("address-%v", i)
		pod := backend.Pod{
			Namespace: "default",
			Name:      fmt.Sprintf("pod-%v", i),
			Address:   address,
		}
		pods[pod] = true
		metrics[pod] = fakeMetrics(i)
	}

	return pods, metrics
}

// fakeMetrics adds numModelsPerPod number of adapters to the pod metrics.
func fakeMetrics(podNumber int) *backend.PodMetrics {
	metrics := &backend.PodMetrics{
		Metrics: backend.Metrics{
			CachedModels: make(map[string]int),
		},
	}
	for i := 0; i < *numModelsPerPod; i++ {
		metrics.CachedModels[modelName(podNumber*(*numModelsPerPod)+i)] = 0
	}
	return metrics
}

func modelName(i int) string {
	return fmt.Sprintf("adapter-%v", i)
}
