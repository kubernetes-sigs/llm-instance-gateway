package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"ext-proc/handlers"
	"ext-proc/metrics"
	"ext-proc/scheduling"

	"github.com/coocood/freecache"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	extProcPb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	healthPb "google.golang.org/grpc/health/grpc_health_v1"
)

type extProcServer struct{}
type server struct{}

var (
	port                              int
	certPath                          string
	enforeFairness                    bool
	cacheActiveLoraModel              *freecache.Cache
	cachePendingRequestActiveAdapters *freecache.Cache
	pods                              []string
	podIPMap                          map[string]string
	ipPodMap                          map[string]string
	interval                          = 30 * time.Second // Update interval for fetching metrics
	TTL                               = int64(7)
)

type healthServer struct{}

func (s *healthServer) Check(ctx context.Context, in *healthPb.HealthCheckRequest) (*healthPb.HealthCheckResponse, error) {
	log.Printf("Handling grpc Check request + %s", in.String())
	return &healthPb.HealthCheckResponse{Status: healthPb.HealthCheckResponse_SERVING}, nil
}

func (s *healthServer) Watch(in *healthPb.HealthCheckRequest, srv healthPb.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "Watch is not implemented")
}

func main() {
	flag.IntVar(&port, "port", 9002, "gRPC port")
	flag.StringVar(&certPath, "certPath", "", "path to extProcServer certificate and private key")
	enforceFairness := flag.Bool("enable-fairness", false, "flag to enable fairness enforcement over the KV-Cache")
	podsFlag := flag.String("pods", "", "Comma-separated list of pod addresses")
	podIPsFlag := flag.String("podIPs", "", "Comma-separated list of pod IPs")
	flag.Parse()

	if *podsFlag == "" || *podIPsFlag == "" {
		log.Fatal("No pods or pod IPs provided. Use the -pods and -podIPs flags to specify comma-separated lists of pod addresses and pod IPs.")
	}

	pods = strings.Split(*podsFlag, ",")
	podIPs := strings.Split(*podIPsFlag, ",")

	if len(pods) != len(podIPs) {
		log.Fatal("The number of pod addresses and pod IPs must match.")
	}

	podIPMap = make(map[string]string)
	for i := range pods {
		podIPMap[pods[i]] = podIPs[i]
	}
	ipPodMap = make(map[string]string)
	for i := range podIPs {
		ipPodMap[podIPs[i]] = pods[i]
	}

	// cache init
	cacheActiveLoraModel = freecache.NewCache(1024)
	cachePendingRequestActiveAdapters = freecache.NewCache(1024)
	debug.SetGCPercent(20)

	// Start the periodic metrics fetching in a separate goroutine

	go metrics.FetchMetricsPeriodically(pods, podIPMap, cacheActiveLoraModel, cachePendingRequestActiveAdapters, interval)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	s := grpc.NewServer()

	extProcPb.RegisterExternalProcessorServer(s, &handlers.Server{
		Pods:                              pods,
		PodIPMap:                          podIPMap,
		IpPodMap:                          ipPodMap,
		CacheActiveLoraModel:              cacheActiveLoraModel,
		CachePendingRequestActiveAdapters: cachePendingRequestActiveAdapters,
		TokenCache:                        scheduling.CreateNewTokenCache(TTL),
		EnforceFairness:                   *enforceFairness,
	})
	healthPb.RegisterHealthServer(s, &healthServer{})

	log.Println("Starting gRPC server on port :9002")

	// shutdown
	var gracefulStop = make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)
	go func() {
		sig := <-gracefulStop
		log.Printf("caught sig: %+v", sig)
		log.Println("Wait for 1 second to finish processing")
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()

	s.Serve(lis)

}
