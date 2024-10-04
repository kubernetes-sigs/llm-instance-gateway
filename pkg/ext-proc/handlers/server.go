package handlers

import (
	"io"

	extProcPb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	klog "k8s.io/klog/v2"

	"ext-proc/backend"
	"ext-proc/scheduling"
)

func NewServer(pp PodProvider, scheduler Scheduler, targetPodHeader string) *Server {
	return &Server{
		scheduler:       scheduler,
		podProvider:     pp,
		targetPodHeader: targetPodHeader,
	}
}

// Server implements the Envoy external processing server.
// https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/ext_proc/v3/external_processor.proto
type Server struct {
	scheduler   Scheduler
	podProvider PodProvider
	// The key of the header to specify the target pod address. This value needs to match Envoy
	// configuration.
	targetPodHeader string
}

type Scheduler interface {
	Schedule(b *scheduling.LLMRequest) (targetPod *backend.Pod, err error)
}

// PodProvider is an interface to provide set of pods in the backend and information such as metrics.
type PodProvider interface {
	GetPodMetrics(pod backend.Pod) (*backend.PodMetrics, bool)
	UpdatePodMetrics(pod backend.Pod, pm *backend.PodMetrics)
}

func (s *Server) Process(srv extProcPb.ExternalProcessor_ProcessServer) error {
	klog.V(2).Info("Processing")
	ctx := srv.Context()
	// Create request context to share states during life time of an HTTP request.
	// See https://github.com/envoyproxy/envoy/issues/17540.
	reqCtx := &RequestContext{}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := srv.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Unknown, "cannot receive stream request: %v", err)
		}

		resp := &extProcPb.ProcessingResponse{}
		switch v := req.Request.(type) {
		case *extProcPb.ProcessingRequest_RequestHeaders:
			resp = HandleRequestHeaders(reqCtx, req)
			klog.V(2).Infof("Request context after HandleRequestHeaders: %v", reqCtx)
		case *extProcPb.ProcessingRequest_RequestBody:
			resp, err = s.HandleRequestBody(reqCtx, req)
			klog.V(2).Infof("Request context after HandleRequestBody: %v", reqCtx)
		case *extProcPb.ProcessingRequest_ResponseHeaders:
			resp, err = s.HandleResponseHeaders(reqCtx, req)
			klog.V(2).Infof("Request context after HandleResponseHeaders: %v", reqCtx)
		default:
			klog.Infof("Unknown Request type %+v", v)
			return status.Error(codes.Unknown, "unknown request type")
		}

		if err != nil {
			klog.Errorf("failed to process request: %v", err)
			return status.Errorf(codes.Unknown, "failed to handle request: %v", err)
		}

		klog.V(2).Infof("response: %v", resp)
		if err := srv.Send(resp); err != nil {
			klog.Infof("send error %v", err)
			return status.Errorf(codes.Unknown, "failed to send response back to Envoy: %v", err)
		}
	}
}

// RequestContext stores context information during the life time of an HTTP request.
type RequestContext struct {
	TargetPod *backend.Pod
	Model     string
}
