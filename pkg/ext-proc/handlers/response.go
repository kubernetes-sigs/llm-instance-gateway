package handlers

import (
	configPb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extProcPb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	klog "k8s.io/klog/v2"
)

// HandleResponseHeaders processes response headers from the backend model server.
func (s *Server) HandleResponseHeaders(reqCtx *RequestContext, req *extProcPb.ProcessingRequest) (*extProcPb.ProcessingResponse, error) {
	klog.V(3).Info("Processing ResponseHeaders")
	h := req.Request.(*extProcPb.ProcessingRequest_ResponseHeaders)
	klog.V(3).Infof("Headers before: %+v\n", h)

	var resp *extProcPb.ProcessingResponse
	if reqCtx.TargetPod != nil {
		resp = &extProcPb.ProcessingResponse{
			Response: &extProcPb.ProcessingResponse_ResponseHeaders{
				ResponseHeaders: &extProcPb.HeadersResponse{
					Response: &extProcPb.CommonResponse{
						HeaderMutation: &extProcPb.HeaderMutation{
							SetHeaders: []*configPb.HeaderValueOption{
								{
									Header: &configPb.HeaderValue{
										// This is for debugging purpose only.
										Key:      "x-went-into-resp-headers",
										RawValue: []byte("true"),
									},
								},
								{
									Header: &configPb.HeaderValue{
										Key:      "target-pod",
										RawValue: []byte(reqCtx.TargetPod.Address),
									},
								},
							},
						},
					},
				},
			},
		}
	} else {
		resp = &extProcPb.ProcessingResponse{
			Response: &extProcPb.ProcessingResponse_ResponseHeaders{
				ResponseHeaders: &extProcPb.HeadersResponse{
					Response: &extProcPb.CommonResponse{
						HeaderMutation: &extProcPb.HeaderMutation{
							SetHeaders: []*configPb.HeaderValueOption{
								{
									Header: &configPb.HeaderValue{
										// This is for debugging purpose only.
										Key:      "x-went-into-resp-headers",
										RawValue: []byte("true"),
									},
								},
							},
						},
					},
				},
			},
		}
	}
	return resp, nil
}
