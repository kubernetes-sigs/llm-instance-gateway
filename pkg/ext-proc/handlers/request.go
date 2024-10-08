package handlers

import (
	"encoding/json"
	"fmt"

	configPb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extProcPb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	klog "k8s.io/klog/v2"

	"ext-proc/scheduling"
)

// HandleRequestBody handles body of the request to the backend server, such as parsing the "model"
// parameter.
// Envoy sends the request body to ext proc before sending the request to the backend server.
func (s *Server) HandleRequestBody(reqCtx *RequestContext, req *extProcPb.ProcessingRequest) (*extProcPb.ProcessingResponse, error) {
	klog.V(3).Infof("Handling request body")

	// Unmarshal request body (must be JSON).
	v := req.Request.(*extProcPb.ProcessingRequest_RequestBody)
	var rb map[string]interface{}
	if err := json.Unmarshal(v.RequestBody.Body, &rb); err != nil {
		klog.Errorf("Error unmarshaling request body: %v", err)
		return nil, fmt.Errorf("error unmarshaling request body: %v", err)
	}
	klog.V(3).Infof("Request body: %v", rb)

	// Resolve target models.
	model, ok := rb["model"].(string)
	if !ok {
		return nil, fmt.Errorf("model not found in request")
	}
	klog.V(3).Infof("Model requested: %v", model)
	llmReq := &scheduling.LLMRequest{
		Model: model,
		// For now use the model as the target model.
		// TODO: Once the API is approved, read the "LLMUseCase" configuration and apply traffic split.
		TargetModels:        map[string]int{model: 100},
		ResolvedTargetModel: model,
	}

	// Update target models in the body.
	rb["model"] = llmReq.ResolvedTargetModel
	updatedBody, err := json.Marshal(rb)
	if err != nil {
		klog.Errorf("Error marshaling request body: %v", err)
		return nil, fmt.Errorf("error marshaling request body: %v", err)
	}
	klog.V(3).Infof("Updated body: %v", updatedBody)

	targetPod, err := s.scheduler.Schedule(llmReq)
	if err != nil {
		return nil, fmt.Errorf("failed to find target pod: %v", err)
	}
	klog.V(3).Infof("Selected target model %v in target pod: %v\n", llmReq.ResolvedTargetModel, targetPod)

	reqCtx.Model = llmReq.Model
	reqCtx.TargetPod = targetPod

	// Insert "target-pod" to instruct Envoy to route requests to the specified target pod.
	headers := []*configPb.HeaderValueOption{
		{
			Header: &configPb.HeaderValue{
				Key:      s.targetPodHeader,
				RawValue: []byte(targetPod.Address),
			},
		},
	}
	// Print headers for debugging
	for _, header := range headers {
		klog.V(3).Infof("[request_body] Header Key: %s, Header Value: %s\n", header.Header.Key, header.Header.RawValue)
	}

	resp := &extProcPb.ProcessingResponse{
		Response: &extProcPb.ProcessingResponse_RequestBody{
			RequestBody: &extProcPb.BodyResponse{
				Response: &extProcPb.CommonResponse{
					HeaderMutation: &extProcPb.HeaderMutation{
						SetHeaders: headers,
					},
					// TODO: Enable body mutation
					// BodyMutation: &extProcPb.BodyMutation{
					// 	Mutation: &extProcPb.BodyMutation_Body{
					// 		Body: updatedBody,
					// 	},
					// },
				},
			},
		},
	}
	return resp, nil
}

func HandleRequestHeaders(reqCtx *RequestContext, req *extProcPb.ProcessingRequest) *extProcPb.ProcessingResponse {
	klog.V(3).Info("--- In RequestHeaders processing ...")
	r := req.Request
	h := r.(*extProcPb.ProcessingRequest_RequestHeaders)
	klog.V(3).Infof("Headers: %+v\n", h)

	resp := &extProcPb.ProcessingResponse{
		Response: &extProcPb.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extProcPb.HeadersResponse{
				Response: &extProcPb.CommonResponse{
					// Set `clear_route_cache = true` to force Envoy to recompute the target cluster
					// based on the new "target-pod" header.
					// See https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/ext_proc/v3/external_processor.proto#service-ext-proc-v3-commonresponse.
					ClearRouteCache: true,
				},
			},
		},
	}

	return resp
}
