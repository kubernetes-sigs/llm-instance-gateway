package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	extProcPb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
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
	certPath               = ""
)

type healthServer struct{}

func (s *healthServer) Check(ctx context.Context, in *healthPb.HealthCheckRequest) (*healthPb.HealthCheckResponse, error) {
	klog.Infof("Handling grpc Check request + %s", in.String())
	return &healthPb.HealthCheckResponse{Status: healthPb.HealthCheckResponse_SERVING}, nil
}

func (s *healthServer) Watch(in *healthPb.HealthCheckRequest, srv healthPb.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "Watch is not implemented")
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	certPool, err := loadCA(certPath)
	if err != nil {
		log.Fatalf("Could not load CA certificate: %v", err)
	}

	// Create TLS configuration
	tlsConfig := &tls.Config{
		RootCAs:    certPool,
		ServerName: "instance-gateway-ext-proc.envoygateway",
	}

	// Create gRPC dial options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	}

	conn, err := grpc.Dial("localhost:9002", opts...)
	if err != nil {
		log.Fatalf("Could not connect: %v", err)
	}
	_ = conn
}

func main() {
	klog.InitFlags(nil)
	flag.StringVar(&certPath, "certPath", "", "path to extProcServer certificate and private key")
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

	creds, err := loadTLSCredentials(certPath)
	if err != nil {
		log.Fatalf("Failed to load TLS credentials: %v", err)
	}

	s := grpc.NewServer(grpc.Creds(creds))

	pp := backend.NewProvider(&vllm.PodMetricsClientImpl{}, &backend.FakePodLister{Pods: pods})
	if err := pp.Init(*refreshPodsInterval, *refreshMetricsInterval); err != nil {
		klog.Fatalf("failed to initialize: %v", err)
	}
	extProcPb.RegisterExternalProcessorServer(s, handlers.NewServer(pp, scheduling.NewScheduler(pp), *targetPodHeader))
	healthPb.RegisterHealthServer(s, &healthServer{})

	klog.Infof("Starting gRPC server on port :%v", *port)

	go func() {
		err = s.Serve(lis)
		if err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	http.HandleFunc("/healthz", healthCheckHandler)
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

}

func loadTLSCredentials(certPath string) (credentials.TransportCredentials, error) {
	// Load extProcServer's certificate and private key
	crt := "server.crt"
	key := "server.key"

	if certPath != "" {
		if !strings.HasSuffix(certPath, "/") {
			certPath = fmt.Sprintf("%s/", certPath)
		}
		crt = fmt.Sprintf("%s%s", certPath, crt)
		key = fmt.Sprintf("%s%s", certPath, key)
	}
	certificate, err := tls.LoadX509KeyPair(crt, key)
	if err != nil {
		return nil, fmt.Errorf("could not load extProcServer key pair: %s", err)
	}

	// Create a new credentials object
	creds := credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{certificate}})

	return creds, nil
}

func loadCA(caPath string) (*x509.CertPool, error) {
	ca := x509.NewCertPool()
	caCertPath := "server.crt"
	if caPath != "" {
		if !strings.HasSuffix(caPath, "/") {
			caPath = fmt.Sprintf("%s/", caPath)
		}
		caCertPath = fmt.Sprintf("%s%s", caPath, caCertPath)
	}
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("could not read ca certificate: %s", err)
	}
	ca.AppendCertsFromPEM(caCert)
	return ca, nil
}
