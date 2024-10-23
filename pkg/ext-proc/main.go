package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	extProcPb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthPb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	klog "k8s.io/klog/v2"

	"ext-proc/backend"
	"ext-proc/backend/vllm"
	"ext-proc/handlers"
	"ext-proc/scheduling"
)

var (
	port            = flag.Int("port", 9002, "gRPC port")
	targetPodHeader = flag.String("targetPodHeader", "target-pod", "the header key for the target pod address to instruct Envoy to send the request to. This must match Envoy configuration.")
	podIPsFlag      = flag.String("podIPs", "", "Comma-separated list of pod IPs")

	refreshPodsInterval    = flag.Duration("refreshPodsInterval", 10*time.Second, "interval to refresh pods")
	refreshMetricsInterval = flag.Duration("refreshMetricsInterval", 50*time.Millisecond, "interval to refresh metrics")
)

type healthServer struct{}

func (s *healthServer) Check(ctx context.Context, in *healthPb.HealthCheckRequest) (*healthPb.HealthCheckResponse, error) {
	klog.Infof("Handling grpc Check request + %s", in.String())
	return &healthPb.HealthCheckResponse{Status: healthPb.HealthCheckResponse_SERVING}, nil
}

func (s *healthServer) Watch(in *healthPb.HealthCheckRequest, srv healthPb.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "Watch is not implemented")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// This is the list of addresses of backend pods.
	// TODO (https://github.com/kubernetes-sigs/llm-instance-gateway/issues/12): Remove this once dynamic pod listing is implemented.
	if *podIPsFlag == "" {
		klog.Fatal("No pods or pod IPs provided. Use the -pods and -podIPs flags to specify comma-separated lists of pod addresses and pod IPs.")
	}
	podIPs := strings.Split(*podIPsFlag, ",")
	klog.Infof("Pods: %v", podIPs)
	pods := make(backend.PodSet)
	for _, ip := range podIPs {
		pod := backend.Pod{
			Namespace: "default",
			Name:      ip,
			Address:   ip,
		}
		pods[pod] = true
	}

	klog.Infof("Listening on %q", fmt.Sprintf(":%d", *port))
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		klog.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	pp := backend.NewProvider(&vllm.PodMetricsClientImpl{}, &backend.FakePodLister{Pods: pods})
	if err := pp.Init(*refreshPodsInterval, *refreshMetricsInterval); err != nil {
		klog.Fatalf("failed to initialize: %v", err)
	}
	extProcPb.RegisterExternalProcessorServer(s, handlers.NewServer(pp, scheduling.NewScheduler(pp), *targetPodHeader))
	healthPb.RegisterHealthServer(s, &healthServer{})

	klog.Infof("Starting gRPC server on port :%v", *port)

	// shutdown
	var gracefulStop = make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)
	go func() {
		sig := <-gracefulStop
		klog.Infof("caught sig: %+v", sig)
		os.Exit(0)
	}()

	s.Serve(lis)

}
