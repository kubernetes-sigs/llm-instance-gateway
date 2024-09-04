package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"ext-proc/cache"
	"ext-proc/metrics"
	"ext-proc/scheduling"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	configPb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	filterPb "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extProcPb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	envoyTypePb "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/coocood/freecache"
)

type Server struct {
	Pods                              []string
	PodIPMap                          map[string]string
	IpPodMap                          map[string]string
	CacheActiveLoraModel              *freecache.Cache
	CachePendingRequestActiveAdapters *freecache.Cache
	TokenCache                        *scheduling.TokenCache
	EnforceFairness                   bool
}

func (s *Server) Process(srv extProcPb.ExternalProcessor_ProcessServer) error {
	log.Println(" ")
	log.Println(" ")
	log.Println("Started process:  -->  ")

	ctx := srv.Context()
	targetPodIP := ""

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

		log.Println(" ")
		log.Println(" ")
		log.Println("Got stream:  -->  ")

		resp := &extProcPb.ProcessingResponse{}
		switch v := req.Request.(type) {
		case *extProcPb.ProcessingRequest_RequestHeaders:
			resp, targetPodIP = s.HandleRequestHeaders(req, targetPodIP)
		case *extProcPb.ProcessingRequest_RequestBody:
			resp, targetPodIP = s.HandleRequestBody(req, targetPodIP)
		case *extProcPb.ProcessingRequest_ResponseHeaders:
			resp, targetPodIP = s.HandleResponseHeaders(req, targetPodIP)
		default:
			log.Printf("Unknown Request type %+v\n", v)
		}

		if err := srv.Send(resp); err != nil {
			log.Printf("send error %v", err)
		}
	}
}

func valueExists(m map[string]string, valueToFind string) bool {
	for _, value := range m {
		if value == valueToFind {
			return true
		}
	}
	return false
}

func (s *Server) HandleRequestBody(req *extProcPb.ProcessingRequest, targetPodIP string) (*extProcPb.ProcessingResponse, string) {
	log.Println("--- In RequestBody processing")
	var requestBody map[string]interface{}
	v := req.Request.(*extProcPb.ProcessingRequest_RequestBody)
	if err := json.Unmarshal(v.RequestBody.Body, &requestBody); err != nil {
		log.Printf("Error unmarshaling request body: %v", err)
		return nil, targetPodIP
	}

	loraAdapterRequested, ok := requestBody["model"].(string)
	if !ok {
		log.Println("model/lora-adapter not found in request body")
		return nil, targetPodIP
	}

	threshold := 100000
	thresholdValue, ok := requestBody["threshold"].(float64)
	if ok {
		threshold = int(thresholdValue)
	}
	targetPod := ""

	if targetPodIP == "" {
		// Retrieve metrics from cache
		var loraMetrics []cache.ActiveLoraModelMetrics
		var requestMetrics []cache.PendingRequestActiveAdaptersMetrics

		for _, pod := range s.Pods {
			loraMetric, err := cache.GetCacheActiveLoraModel(s.CacheActiveLoraModel, pod, loraAdapterRequested)
			if err == nil {
				loraMetrics = append(loraMetrics, *loraMetric)
			} else if err != freecache.ErrNotFound {
				log.Printf("Error fetching cacheActiveLoraModel for pod %s and lora_adapter_requested %s: %v", pod, loraAdapterRequested, err)
			}

			requestMetric, err := cache.GetCachePendingRequestActiveAdapters(s.CachePendingRequestActiveAdapters, pod)
			if err == nil {
				requestMetrics = append(requestMetrics, *requestMetric)
			} else if err != freecache.ErrNotFound {
				log.Printf("Error fetching cachePendingRequestActiveAdapters for pod %s: %v", pod, err)
				break
			}
		}

		fmt.Printf("Fetched loraMetrics: %+v\n", loraMetrics)
		fmt.Printf("Fetched requestMetrics: %+v\n", requestMetrics)

		targetPod = metrics.FindTargetPod(loraMetrics, requestMetrics, loraAdapterRequested, threshold)
		targetPodIP = s.PodIPMap[targetPod]
		fmt.Printf("Selected target pod: %s\n", targetPod)
		fmt.Printf("Selected target pod IP: %s\n", targetPodIP)
	} else {
		targetPod = s.IpPodMap[targetPodIP]
		fmt.Printf("Pre-selected target pod: %s\n", targetPod)
		fmt.Printf("Pre-selected target pod IP: %s\n", targetPodIP)
	}

	var resp *extProcPb.ProcessingResponse
	if s.EnforceFairness && !s.TokenCache.IsFairRequest(loraAdapterRequested) {
		resp = &extProcPb.ProcessingResponse{
			Response: &extProcPb.ProcessingResponse_ImmediateResponse{
				ImmediateResponse: &extProcPb.ImmediateResponse{
					Status: &envoyTypePb.HttpStatus{
						Code: envoyTypePb.StatusCode_TooManyRequests,
					},
				},
			},
		}
	} else if !metrics.Contains(s.Pods, targetPod) {
		resp = &extProcPb.ProcessingResponse{
			Response: &extProcPb.ProcessingResponse_ImmediateResponse{
				ImmediateResponse: &extProcPb.ImmediateResponse{
					Status: &envoyTypePb.HttpStatus{
						Code: envoyTypePb.StatusCode_NotFound,
					},
				},
			},
		}
	} else {
		headers := []*configPb.HeaderValueOption{
			{
				Header: &configPb.HeaderValue{
					Key:      "x-went-into-req-body",
					RawValue: []byte("true"),
				},
			},
			{
				Header: &configPb.HeaderValue{
					Key:      "target-pod",
					RawValue: []byte(targetPodIP),
				},
			},
		}

		// Print headers
		for _, header := range headers {
			fmt.Printf("[request_body] Header Key: %s, Header Value: %s\n", header.Header.Key, header.Header.RawValue)
		}

		resp = &extProcPb.ProcessingResponse{
			Response: &extProcPb.ProcessingResponse_RequestBody{
				RequestBody: &extProcPb.BodyResponse{
					Response: &extProcPb.CommonResponse{
						HeaderMutation: &extProcPb.HeaderMutation{
							SetHeaders: headers,
						},
					},
				},
			},
		}
	}
	return resp, targetPodIP
}

func (s *Server) HandleResponseHeaders(req *extProcPb.ProcessingRequest, targetPodIP string) (*extProcPb.ProcessingResponse, string) {
	log.Println("--- In ResponseHeaders processing")
	r := req.Request
	h := r.(*extProcPb.ProcessingRequest_ResponseHeaders)

	log.Printf("Headers: %+v\n", h)

	var loraMetrics []cache.ActiveLoraModelMetrics
	var requestMetrics []cache.PendingRequestActiveAdaptersMetrics
	var modelNames map[string]int
	var totalTokens int
	var model string
	var err error
	currentTime := time.Now().Unix()
	pendingQueueSize := -1
	podAdapterMap := make(map[string]int)
	targetPod := s.IpPodMap[targetPodIP]
	for _, header := range h.ResponseHeaders.Headers.Headers {
		switch header.Key {
		case "active_lora_adapters":
			err = json.Unmarshal([]byte(header.RawValue), &modelNames)
			if err != nil {
				log.Printf("Error parsing model_names: %v", err)
			}
		case "pending_queue_size":
			var err error
			pendingQueueSize, err = strconv.Atoi(string(header.RawValue))
			if err != nil {
				log.Printf("Error converting pending_queue_size: %v", err)
			}
		case "model":
			model = string(header.RawValue)
		case "total_tokens":
			totalTokens, err = strconv.Atoi(string(header.RawValue))
			if err != nil {
				log.Printf("Error parsing total_tokens: %v", err)
			}
		}
	}
	if modelNames != nil {
		for modelName, numberOfPendingRequests := range modelNames {
			metric := cache.ActiveLoraModelMetrics{
				Date:                    time.Now().Format(time.RFC3339),
				PodName:                 targetPod,
				ModelName:               modelName,
				NumberOfPendingRequests: numberOfPendingRequests,
			}
			podAdapterMap[metric.PodName]++
			loraMetrics = append(loraMetrics, metric)
		}
		// Update cache with parsed values
		for _, metric := range loraMetrics {
			if err := cache.SetCacheActiveLoraModel(s.CacheActiveLoraModel, metric); err != nil {
				log.Printf("Error setting cache in Response Header: %v", err)
			}
		}
	}
	if pendingQueueSize >= 0 {
		requestMetric := cache.PendingRequestActiveAdaptersMetrics{
			Date:                   time.Now().Format(time.RFC3339),
			PodName:                targetPod,
			PendingRequests:        pendingQueueSize,
			NumberOfActiveAdapters: podAdapterMap[targetPod],
		}
		requestMetrics = append(requestMetrics, requestMetric)
		for _, metric := range requestMetrics {
			if err := cache.SetCachePendingRequestActiveAdapters(s.CachePendingRequestActiveAdapters, metric); err != nil {
				log.Printf("Error setting cache in Response Header: %v", err)
			}
		}
	}
	log.Printf("Model Value: %v", model)
	log.Printf("Total Tokens: %v", totalTokens)
	if "model" != "" {
		s.TokenCache.StoreResponseInfo(model, currentTime, totalTokens)
	}
	s.TokenCache.AdapterMap.Range(func(k, v any) bool {
		log.Printf("Adapter: %+v Entries: %+v", k, v)
		return true
	})

	resp := &extProcPb.ProcessingResponse{
		Response: &extProcPb.ProcessingResponse_ResponseHeaders{
			ResponseHeaders: &extProcPb.HeadersResponse{
				Response: &extProcPb.CommonResponse{
					HeaderMutation: &extProcPb.HeaderMutation{
						SetHeaders: []*configPb.HeaderValueOption{
							{
								Header: &configPb.HeaderValue{
									Key:      "x-went-into-resp-headers",
									RawValue: []byte("true"),
								},
							},
							{
								Header: &configPb.HeaderValue{
									Key:      "target-pod",
									RawValue: []byte(targetPod),
								},
							},
						},
					},
				},
			},
		},
	}
	return resp, targetPod
}

func (s *Server) HandleRequestHeaders(req *extProcPb.ProcessingRequest, targetPodIP string) (*extProcPb.ProcessingResponse, string) {
	log.Println("--- In RequestHeaders processing ...")
	r := req.Request
	h := r.(*extProcPb.ProcessingRequest_RequestHeaders)

	log.Printf("Headers: %+v\n", h)
	log.Printf("EndOfStream: %v\n", h.RequestHeaders.EndOfStream)
	for _, n := range h.RequestHeaders.Headers.Headers {
		if strings.ToLower(n.Key) == "target-pod" {
			targetPodIP = string(n.RawValue)
		}
	}

	var resp *extProcPb.ProcessingResponse
	if targetPodIP == "" {
		bodyMode := filterPb.ProcessingMode_BUFFERED

		resp = &extProcPb.ProcessingResponse{
			Response: &extProcPb.ProcessingResponse_RequestHeaders{
				RequestHeaders: &extProcPb.HeadersResponse{
					Response: &extProcPb.CommonResponse{
						HeaderMutation: &extProcPb.HeaderMutation{
							SetHeaders: []*configPb.HeaderValueOption{
								{
									Header: &configPb.HeaderValue{
										Key:      "x-went-into-req-headers",
										RawValue: []byte("true"),
									},
								},
							},
						},
						ClearRouteCache: true,
					},
				},
			},
			ModeOverride: &filterPb.ProcessingMode{
				ResponseHeaderMode: filterPb.ProcessingMode_SEND,
				RequestBodyMode:    bodyMode,
			},
		}
	} else {
		bodyMode := filterPb.ProcessingMode_NONE

		resp = &extProcPb.ProcessingResponse{
			Response: &extProcPb.ProcessingResponse_RequestHeaders{
				RequestHeaders: &extProcPb.HeadersResponse{
					Response: &extProcPb.CommonResponse{
						HeaderMutation: &extProcPb.HeaderMutation{
							SetHeaders: []*configPb.HeaderValueOption{
								{
									Header: &configPb.HeaderValue{
										Key:      "x-went-into-req-headers",
										RawValue: []byte("true"),
									},
								},
								{
									Header: &configPb.HeaderValue{
										Key:      "target-pod",
										RawValue: []byte(targetPodIP),
									},
								},
							},
						},
						ClearRouteCache: true,
					},
				},
			},
			ModeOverride: &filterPb.ProcessingMode{
				ResponseHeaderMode: filterPb.ProcessingMode_SEND,
				RequestBodyMode:    bodyMode,
			},
		}
	}
	// Print final headers being sent
	fmt.Println("[request_header]Final headers being sent:")
	for _, header := range resp.GetRequestHeaders().GetResponse().GetHeaderMutation().GetSetHeaders() {
		fmt.Printf("%s: %s\n", header.GetHeader().Key, header.GetHeader().RawValue)
	}
	return resp, targetPodIP
}
