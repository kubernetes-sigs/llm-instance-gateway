package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	extProcPb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthPb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	// K8s imports
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	klog "k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	// I-GW imports
	"inference.networking.x-k8s.io/llm-instance-gateway/api/v1alpha1"
	clientset "inference.networking.x-k8s.io/llm-instance-gateway/client-go/clientset/versioned"
	informers "inference.networking.x-k8s.io/llm-instance-gateway/client-go/informers/externalversions"
	"inference.networking.x-k8s.io/llm-instance-gateway/pkg/ext-proc/backend"
	"inference.networking.x-k8s.io/llm-instance-gateway/pkg/ext-proc/backend/vllm"
	"inference.networking.x-k8s.io/llm-instance-gateway/pkg/ext-proc/handlers"
	"inference.networking.x-k8s.io/llm-instance-gateway/pkg/ext-proc/scheduling"
)

var (
	port                   = flag.Int("port", 9002, "gRPC port")
	targetPodHeader        = flag.String("targetPodHeader", "target-pod", "the header key for the target pod address to instruct Envoy to send the request to. This must match Envoy configuration.")
	serverPoolName         = flag.String("serverPoolName", "", "Name of the serverPool this Endpoint Picker is associated with.")
	serviceName            = flag.String("serviceName", "", "Name of the service that will be used to read the endpointslices from")
	namespace              = flag.String("namespace", "default", "The Namespace that the server pool should exist in.")
	zone                   = flag.String("zone", "", "The zone that this instance is created in. Will be passed to the corresponding endpointSlice. ")
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

func init() {
	utilruntime.Must(scheme.AddToScheme(scheme.Scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme.Scheme))
}

func main() {

	klog.InitFlags(nil)
	flag.Parse()

	ctrl.SetLogger(klog.TODO())
	ctx := SetupSignalHandler()

	klog.Infof("Listening on %q", fmt.Sprintf(":%d", *port))
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		klog.Fatalf("failed to listen: %v", err)
	}

	datastore := &backend.K8sDatastore{
		LLMServerPool: &v1alpha1.LLMServerPool{},
		LLMServices:   &sync.Map{},
		Pods:          &sync.Map{},
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		klog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Error(err, "Error building kubernetes clientset")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Error(err, "Error building kubernetes clientset")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	poolInformerFactory := informers.NewSharedInformerFactory(client, time.Second*30)

	poolReconciler := backend.NewLLMServerPoolReconciler(
		ctx,
		poolInformerFactory.Api().V1alpha1().LLMServerPools(),
		client,
		kubeClient,
		scheme.Scheme,
		*serverPoolName,
		*namespace,
		*zone,
		datastore,
	)

	// This is required, and informers will not sync without this func being called.
	poolInformerFactory.Start(ctx.Done())

	errChan := make(chan error)
	go func() {
		if err := poolReconciler.Run(ctx, 2); err != nil {
			errChan <- err
		}
	}()

	if err := (&backend.LLMServiceReconciler{
		Datastore:      datastore,
		Scheme:         mgr.GetScheme(),
		Client:         mgr.GetClient(),
		ServerPoolName: *serverPoolName,
		Namespace:      *namespace,
		Record:         mgr.GetEventRecorderFor("llmservice"),
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "Error setting up LLMServiceReconciler")
	}

	if err := (&backend.EndpointSliceReconciler{
		Datastore:      datastore,
		Scheme:         mgr.GetScheme(),
		Client:         mgr.GetClient(),
		Record:         mgr.GetEventRecorderFor("endpointslice"),
		ServiceName:    *serviceName,
		Zone:           *zone,
		ServerPoolName: *serverPoolName,
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "Error setting up EndpointSliceReconciler")
	}

	go func() {
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			klog.Error(err, "Error running manager")
			errChan <- err
		}
	}()

	s := grpc.NewServer()

	pp := backend.NewProvider(&vllm.PodMetricsClientImpl{}, datastore)
	if err := pp.Init(*refreshPodsInterval, *refreshMetricsInterval); err != nil {
		klog.Fatalf("failed to initialize: %v", err)
	}
	extProcPb.RegisterExternalProcessorServer(s, handlers.NewServer(pp, scheduling.NewScheduler(pp), *targetPodHeader, datastore))
	healthPb.RegisterHealthServer(s, &healthServer{})

	klog.Infof("Starting gRPC server on port :%v", *port)

	// shutdown
	var gracefulStop = make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)
	go func() {
		select {
		case sig := <-gracefulStop:
			klog.Infof("caught sig: %+v", sig)
			os.Exit(0)
		case err := <-errChan:
			klog.Infof("caught error in controller: %+v", err)
			os.Exit(0)
		}

	}()

	s.Serve(lis)

}

var onlyOneSignalHandler = make(chan struct{})

// SetupSignalHandler registered for SIGTERM and SIGINT. A context is returned
// which is cancelled on one of these signals. If a second signal is caught,
// the program is terminated with exit code 1.
func SetupSignalHandler() context.Context {
	close(onlyOneSignalHandler) // panics when called twice

	c := make(chan os.Signal, 2)
	ctx, cancel := context.WithCancel(context.Background())
	shutdownSignals := []os.Signal{os.Interrupt, syscall.SIGTERM}
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return ctx
}
