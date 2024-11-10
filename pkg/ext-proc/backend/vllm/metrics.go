// Package vllm provides vllm specific pod metrics implementation.
package vllm

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"inference.networking.x-k8s.io/llm-instance-gateway/pkg/ext-proc/backend"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"go.uber.org/multierr"
	klog "k8s.io/klog/v2"
)

const (
	ActiveLoRAAdaptersMetricName        = "vllm:info_active_adapters_info"
	LoraRequestInfoMetricName           = "vllm:lora_requests_info"
	LoRAAdapterPendingRequestMetricName = "vllm:active_lora_adapters"
	// TODO: Replace these with the num_tokens_running/waiting below once we add those to the fork.
	RunningQueueSizeMetricName = "vllm:num_requests_running"
	WaitingQueueSizeMetricName = "vllm:num_requests_waiting"
	/* TODO: Uncomment this once the following are added to the fork.
	RunningQueueSizeMetricName        = "vllm:num_tokens_running"
	WaitingQueueSizeMetricName        = "vllm:num_tokens_waiting"
	*/
	KVCacheUsagePercentMetricName     = "vllm:gpu_cache_usage_perc"
	KvCacheMaxTokenCapacityMetricName = "vllm:gpu_cache_max_token_capacity"
)

type PodMetricsClientImpl struct {
}

// FetchMetrics fetches metrics from a given pod.
func (p *PodMetricsClientImpl) FetchMetrics(ctx context.Context, pod backend.Pod, existing *backend.PodMetrics) (*backend.PodMetrics, error) {
	// Currently the metrics endpoint is hard-coded, which works with vLLM.
	// TODO(https://github.com/kubernetes-sigs/llm-instance-gateway/issues/16): Consume this from LLMServerPool config.
	url := fmt.Sprintf("http://%s/metrics", pod.Address)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		klog.Errorf("failed to fetch metrics from %s: %v", pod, err)
		return nil, fmt.Errorf("failed to fetch metrics from %s: %w", pod, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		klog.Errorf("unexpected status code from %s: %v", pod, resp.StatusCode)
		return nil, fmt.Errorf("unexpected status code from %s: %v", pod, resp.StatusCode)
	}

	parser := expfmt.TextParser{}
	metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, err
	}
	return promToPodMetrics(metricFamilies, existing)
}

// promToPodMetrics updates internal pod metrics with scraped prometheus metrics.
// A combined error is returned if errors occur in one or more metric processing.
// it returns a new PodMetrics pointer which can be used to atomically update the pod metrics map.
func promToPodMetrics(metricFamilies map[string]*dto.MetricFamily, existing *backend.PodMetrics) (*backend.PodMetrics, error) {
	var errs error
	updated := existing.Clone()
	runningQueueSize, _, err := getLatestMetric(metricFamilies, RunningQueueSizeMetricName)
	errs = multierr.Append(errs, err)
	if err == nil {
		updated.RunningQueueSize = int(runningQueueSize.GetGauge().GetValue())
	}
	waitingQueueSize, _, err := getLatestMetric(metricFamilies, WaitingQueueSizeMetricName)
	errs = multierr.Append(errs, err)
	if err == nil {
		updated.WaitingQueueSize = int(waitingQueueSize.GetGauge().GetValue())
	}
	cachePercent, _, err := getLatestMetric(metricFamilies, KVCacheUsagePercentMetricName)
	errs = multierr.Append(errs, err)
	if err == nil {
		updated.KVCacheUsagePercent = cachePercent.GetGauge().GetValue()
	}

	loraMetrics, _, err := getLatestLoraMetric(metricFamilies[LoraRequestInfoMetricName])
	multierr.Append(errs, err)
	/* TODO: uncomment once this is available in vllm.
	kvCap, _, err := getGaugeLatestValue(metricFamilies, KvCacheMaxTokenCapacityMetricName)
	errs = multierr.Append(errs, err)
	if err != nil {
		updated.KvCacheMaxTokenCapacity = int(kvCap)
	}
	*/

	// TODO(https://github.com/kubernetes-sigs/llm-instance-gateway/issues/22): Read from vLLM metrics once the is available.
	updated.MaxActiveModels = 4

	// Update active loras
	mf, ok := metricFamilies[ActiveLoRAAdaptersMetricName]
	if ok {
		// IMPORTANT: replace the map entries instead of appending to it.
		updated.ActiveModels = make(map[string]int)
		for _, metric := range mf.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "active_adapters" {
					if label.GetValue() != "" {
						adapterList := strings.Split(label.GetValue(), ",")
						for _, adapter := range adapterList {
							updated.ActiveModels[adapter] = 0
						}
					}
				}
			}
		}
	} else {
		klog.Warningf("metric family %q not found", ActiveLoRAAdaptersMetricName)
		multierr.Append(errs, fmt.Errorf("metric family %q not found", ActiveLoRAAdaptersMetricName))
	}

	if loraMetrics != nil {
		updated.Metrics.ActiveModels = make(map[string]int)
		for _, label := range loraMetrics.GetLabel() {
			if label.GetName() == "running_lora_adapters" {
				if label.GetValue() != "" {
					adapterList := strings.Split(label.GetValue(), ",")
					for _, adapter := range adapterList {
						updated.Metrics.ActiveModels[adapter] = 0
					}
				}
			}
		}

	}

	if loraMetrics != nil {
		updated.CachedModels = make(map[string]int)
		for _, label := range loraMetrics.GetLabel() {
			if label.GetName() == "running_lora_adapters" {
				if label.GetValue() != "" {
					adapterList := strings.Split(label.GetValue(), ",")
					for _, adapter := range adapterList {
						updated.CachedModels[adapter] = 0
					}
				}
			}
		}

	}

	return updated, errs
}

func getLatestLoraMetric(loraRequests *dto.MetricFamily) (*dto.Metric, time.Time, error) {
	var latestTs float64
	var latest *dto.Metric
	for _, m := range loraRequests.GetMetric() {
		if m.GetGauge().GetValue() > latestTs {
			latestTs = m.GetGauge().GetValue()
			latest = m
		}
	}
	return latest, time.Unix(0, int64(latestTs*1000)), nil
}

// getLatestMetric gets the latest metric of a family. This should be used to get the latest Gauge metric.
func getLatestMetric(metricFamilies map[string]*dto.MetricFamily, metricName string) (*dto.Metric, time.Time, error) {
	mf, ok := metricFamilies[metricName]
	if !ok {
		klog.Warningf("metric family %q not found", metricName)
		return nil, time.Time{}, fmt.Errorf("metric family %q not found", metricName)
	}
	if len(mf.GetMetric()) == 0 {
		return nil, time.Time{}, fmt.Errorf("no metrics available for %q", metricName)
	}
	var latestTs int64
	var latest *dto.Metric
	for _, m := range mf.GetMetric() {
		if m.GetTimestampMs() >= latestTs {
			latestTs = m.GetTimestampMs()
			latest = m
		}
	}
	klog.V(4).Infof("Got metric value %+v for metric %v", latest, metricName)
	return latest, time.Unix(0, latestTs*1000), nil
}
