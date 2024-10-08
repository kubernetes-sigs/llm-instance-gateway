package backend

import (
	"fmt"
	"strings"
	"sync"
	"time"

	dto "github.com/prometheus/client_model/go"
	"go.uber.org/multierr"
	klog "k8s.io/klog/v2"
)

const (
	ActiveLoRAAdaptersMetricName        = "vllm:info_active_adapters_info"
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

func (p *Provider) refreshMetricsOnce() error {
	start := time.Now()
	defer func() {
		d := time.Now().Sub(start)
		// TODO: add a metric instead of logging
		klog.V(4).Infof("Refreshed metrics in %v", d)
	}()
	var wg sync.WaitGroup
	var errs error
	processOnePod := func(key, value any) bool {
		klog.V(4).Infof("Processing pod %v and metric %v", key, value)
		pod := key.(Pod)
		metrics := value.(*PodMetrics)
		wg.Add(1)
		go func() {
			defer wg.Done()
			metricFamilies, err := p.pmc.FetchMetrics(pod)
			if err != nil {
				multierr.Append(errs, fmt.Errorf("failed to parse metrics from %s: %v", pod, err))
				return
			}
			updated, err := promToPodMetrics(metricFamilies, metrics)
			klog.V(4).Infof("Updated metrics for pod %s: %v", pod, updated.Metrics)
			if err != nil {
				multierr.Append(errs, fmt.Errorf("failed to get all pod metrics updated from prometheus: %v", err))
			}
			p.UpdatePodMetrics(pod, updated)
		}()
		return true
	}
	p.podMetrics.Range(processOnePod)
	wg.Wait()
	return errs
}

// promToPodMetrics updates internal pod metrics with scraped prometheus metrics.
// A combined error is returned if errors occur in one or more metric processing.
// it returns a new PodMetrics pointer which can be used to atomically update the pod metrics map.
func promToPodMetrics(metricFamilies map[string]*dto.MetricFamily, existing *PodMetrics) (*PodMetrics, error) {
	var errs error
	updated := existing.Clone()
	runningQueueSize, _, err := getLatestMetric(metricFamilies, RunningQueueSizeMetricName)
	multierr.Append(errs, err)
	if err == nil {
		updated.RunningQueueSize = int(runningQueueSize.GetGauge().GetValue())
	}
	waitingQueueSize, _, err := getLatestMetric(metricFamilies, WaitingQueueSizeMetricName)
	multierr.Append(errs, err)
	if err == nil {
		updated.WaitingQueueSize = int(waitingQueueSize.GetGauge().GetValue())
	}
	cachePercent, _, err := getLatestMetric(metricFamilies, KVCacheUsagePercentMetricName)
	multierr.Append(errs, err)
	if err == nil {
		updated.KVCacheUsagePercent = cachePercent.GetGauge().GetValue()
	}
	/* TODO: uncomment once this is available in vllm.
	kvCap, _, err := getGaugeLatestValue(metricFamilies, KvCacheMaxTokenCapacityMetricName)
	multierr.Append(errs, err)
	if err != nil {
		updated.KvCacheMaxTokenCapacity = int(kvCap)
	}
	*/

	// Update active loras
	mf, ok := metricFamilies[ActiveLoRAAdaptersMetricName]
	if ok {
		// IMPORTANT: replace the map entries instead of appending to it.
		updated.CachedModels = make(map[string]int)
		for _, metric := range mf.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "active_adapters" {
					if label.GetValue() != "" {
						adapterList := strings.Split(label.GetValue(), ",")
						for _, adapter := range adapterList {
							updated.CachedModels[adapter] = 0
						}
					}
				}
			}
		}
	} else {
		klog.Warningf("metric family %q not found", ActiveLoRAAdaptersMetricName)
		multierr.Append(errs, fmt.Errorf("metric family %q not found", ActiveLoRAAdaptersMetricName))
	}

	return updated, errs
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
