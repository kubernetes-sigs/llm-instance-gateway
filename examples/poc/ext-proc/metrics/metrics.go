package metrics

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coocood/freecache"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	"ext-proc/cache"
)

// Contains checks if a slice contains a specific element
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// FetchLoraMetricsFromPod fetches metrics from a given pod and sends them to a channel
func FetchLoraMetricsFromPod(pod string, podIPMap map[string]string, ch chan<- []cache.ActiveLoraModelMetrics, wg *sync.WaitGroup) {
	defer wg.Done()
	ip, exists := podIPMap[pod]
	if !exists {
		log.Printf("pod %s has no corresponding ip defined", pod)
		return
	}
	url := fmt.Sprintf("http://%s/metrics", ip)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("failed to fetch metrics from %s: %v", pod, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("unexpected status code from %s: %v", pod, resp.StatusCode)
		return
	}

	parser := expfmt.TextParser{}
	metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		log.Printf("failed to parse metrics from %s: %v", pod, err)
		return
	}

	var loraMetrics []cache.ActiveLoraModelMetrics
	var adapterList []string
	modelsDict := make(map[string]int)

	for name, mf := range metricFamilies {
		if name == "vllm:active_lora_adapters" {
			for _, m := range mf.GetMetric() {
				modelName := GetLabelValue(m, "active_lora_adapters")
				numberOfPendingRequests := int(m.GetGauge().GetValue())
				modelsDict[modelName] = numberOfPendingRequests
			}
		}
		if name == "vllm:info_active_adapters_info" {
			for _, metric := range mf.GetMetric() {
				for _, label := range metric.GetLabel() {
					if label.GetName() == "active_adapters" {
						if label.GetValue() != "" {
							adapterList = strings.Split(label.GetValue(), ",")
						}
					}
				}
			}
		}
	}

	for modelName, numberOfPendingRequests := range modelsDict {
		if !Contains(adapterList, modelName) {
			continue
		}
		loraMetric := cache.ActiveLoraModelMetrics{
			Date:                    time.Now().Format(time.RFC3339),
			PodName:                 pod,
			ModelName:               modelName,
			NumberOfPendingRequests: numberOfPendingRequests,
		}
		loraMetrics = append(loraMetrics, loraMetric)
	}

	ch <- loraMetrics
}

// FetchRequestMetricsFromPod fetches request metrics from a given pod and sends them to a channel
func FetchRequestMetricsFromPod(pod string, podIPMap map[string]string, ch chan<- []cache.PendingRequestActiveAdaptersMetrics, wg *sync.WaitGroup) {
	defer wg.Done()

	ip, exists := podIPMap[pod]
	if !exists {
		log.Printf("pod %s has no corresponding ip defined", pod)
		return
	}
	url := fmt.Sprintf("http://%s/metrics", ip)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("failed to fetch metrics from %s: %v", pod, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("unexpected status code from %s: %v", pod, resp.StatusCode)
		return
	}

	parser := expfmt.TextParser{}
	metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		log.Printf("failed to parse metrics from %s: %v", pod, err)
		return
	}

	var requestMetrics []cache.PendingRequestActiveAdaptersMetrics
	pendingRequests := 0
	adapterCount := 0

	for name, mf := range metricFamilies {
		switch name {
		case "vllm:num_requests_waiting":
			for _, m := range mf.GetMetric() {
				pendingRequests += int(m.GetGauge().GetValue())
			}
		case "vllm:num_requests_running":
			for _, m := range mf.GetMetric() {
				pendingRequests += int(m.GetGauge().GetValue())
			}
		case "vllm:info_active_adapters_info":
			for _, metric := range mf.GetMetric() {
				for _, label := range metric.GetLabel() {
					if label.GetName() == "active_adapters" {
						if label.GetValue() != "" {
							adapterCount = len(strings.Split(label.GetValue(), ","))
						}
					}
				}
			}
		}
	}

	requestMetric := cache.PendingRequestActiveAdaptersMetrics{
		Date:                   time.Now().Format(time.RFC3339),
		PodName:                pod,
		PendingRequests:        pendingRequests,
		NumberOfActiveAdapters: adapterCount,
	}
	requestMetrics = append(requestMetrics, requestMetric)

	ch <- requestMetrics
}

// FetchMetrics fetches metrics from all pods and returns them
func FetchMetrics(pods []string, podIPMap map[string]string) ([]cache.ActiveLoraModelMetrics, []cache.PendingRequestActiveAdaptersMetrics) {
	ch := make(chan []cache.ActiveLoraModelMetrics)
	ch2 := make(chan []cache.PendingRequestActiveAdaptersMetrics)
	var wg sync.WaitGroup
	var wg2 sync.WaitGroup

	for _, pod := range pods {
		wg.Add(1)
		go FetchLoraMetricsFromPod(pod, podIPMap, ch, &wg)
	}

	for _, pod := range pods {
		wg2.Add(1)
		go FetchRequestMetricsFromPod(pod, podIPMap, ch2, &wg2)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	go func() {
		wg2.Wait()
		close(ch2)
	}()

	var allLoraMetrics []cache.ActiveLoraModelMetrics
	var allRequestMetrics []cache.PendingRequestActiveAdaptersMetrics
	for loraMetrics := range ch {
		allLoraMetrics = append(allLoraMetrics, loraMetrics...)
	}
	for requestMetrics := range ch2 {
		allRequestMetrics = append(allRequestMetrics, requestMetrics...)
	}
	return allLoraMetrics, allRequestMetrics
}

// GetLabelValue returns the value of a label from a Prometheus metric
func GetLabelValue(m *io_prometheus_client.Metric, label string) string {
	for _, l := range m.GetLabel() {
		if l.GetName() == label {
			return l.GetValue()
		}
	}
	return ""
}

// FindTargetPod finds the target pod based on metrics and the requested lora adapter
func FindTargetPod(loraMetrics []cache.ActiveLoraModelMetrics, requestMetrics []cache.PendingRequestActiveAdaptersMetrics, loraAdapterRequested string, threshold int) string {
	var targetPod string
	bestAlternativePod := ""
	minAltRequests := math.MaxInt

	fmt.Println("Searching for the best pod...")

	// Filter metrics for the requested model
	for _, reqMetric := range requestMetrics {
		if reqMetric.PendingRequests < minAltRequests {
			minAltRequests = reqMetric.PendingRequests
			bestAlternativePod = reqMetric.PodName
		}
	}

	if loraAdapterRequested == "" {
		targetPod = bestAlternativePod
		if targetPod == "" {
			fmt.Println("Error: No pod found")
		} else {
			fmt.Printf("Selected the best alternative pod: %s with %d pending requests\n", targetPod, minAltRequests)
		}
		return targetPod
	}

	var relevantMetrics []cache.ActiveLoraModelMetrics
	for _, metric := range loraMetrics {
		if metric.ModelName == loraAdapterRequested {
			relevantMetrics = append(relevantMetrics, metric)
		}
	}

	// If no metrics found for the requested model, choose the pod with the least active adapters randomly
	if len(relevantMetrics) == 0 {
		minActiveAdapters := math.MaxInt
		var podsWithLeastAdapters []cache.PendingRequestActiveAdaptersMetrics
		for _, reqMetric := range requestMetrics {
			if reqMetric.NumberOfActiveAdapters < minActiveAdapters {
				minActiveAdapters = reqMetric.NumberOfActiveAdapters
				podsWithLeastAdapters = []cache.PendingRequestActiveAdaptersMetrics{}
			}
			if reqMetric.NumberOfActiveAdapters == minActiveAdapters {
				podsWithLeastAdapters = append(podsWithLeastAdapters, reqMetric)
			}
		}

		if len(podsWithLeastAdapters) == 0 {
			fmt.Println("Error: No pod with min adapter found")
		} else {
			rand.Seed(time.Now().UnixNano())
			targetPod = podsWithLeastAdapters[rand.Intn(len(podsWithLeastAdapters))].PodName
			fmt.Printf("Selected pod with the least active adapters: %s\n", targetPod)
		}
		return targetPod
	}

	// Find the pod with the max lora requests among the relevant metrics
	maxNumberOfPendingRequests := -1
	var bestPods []cache.ActiveLoraModelMetrics
	for _, metric := range relevantMetrics {
		if metric.ModelName == loraAdapterRequested {
			if metric.NumberOfPendingRequests > maxNumberOfPendingRequests {
				maxNumberOfPendingRequests = metric.NumberOfPendingRequests
				bestPods = []cache.ActiveLoraModelMetrics{}
			}
			if metric.NumberOfPendingRequests == maxNumberOfPendingRequests {
				bestPods = append(bestPods, metric)
			}
		}
	}

	if len(bestPods) > 0 {
		rand.Seed(time.Now().UnixNano())
		targetPod = bestPods[rand.Intn(len(bestPods))].PodName
		fmt.Printf("Selected pod with the highest NumberOfPendingRequests: %s\n", targetPod)
	} else {
		fmt.Printf("No pods match the requested model: %s\n", loraAdapterRequested)
	}

	// If the number of active Lora adapters in the selected pod is greater than the threshold, choose the pod with the least requests
	if maxNumberOfPendingRequests > threshold && bestAlternativePod != "" {
		targetPod = bestAlternativePod
		fmt.Printf("Selected pod's active Lora adapters exceed threshold, selecting the best alternative pod: %s with %d pending requests\n", targetPod, minAltRequests)
	}

	if targetPod == "" {
		fmt.Println("Error: No pod found")
	}

	return targetPod
}

// FetchMetricsPeriodically fetches metrics periodically and updates the cache
func FetchMetricsPeriodically(pods []string, podIPMap map[string]string, cacheActiveLoraModel *freecache.Cache, cachePendingRequestActiveAdapters *freecache.Cache, interval time.Duration) {
	for {
		loraMetrics, requestMetrics := FetchMetrics(pods, podIPMap)
		fmt.Printf("fetchMetricsPeriodically requestMetrics: %+v\n", requestMetrics)
		fmt.Printf("fetchMetricsPeriodically loraMetrics: %+v\n", loraMetrics)
		cacheActiveLoraModel.Clear()
		cachePendingRequestActiveAdapters.Clear()
		for _, metric := range loraMetrics {
			if err := cache.SetCacheActiveLoraModel(cacheActiveLoraModel, metric); err != nil {
				log.Printf("Error setting cache: %v", err)
			}
		}
		for _, metric := range requestMetrics {
			if err := cache.SetCachePendingRequestActiveAdapters(cachePendingRequestActiveAdapters, metric); err != nil {
				log.Printf("Error setting cache: %v", err)
			}
		}
		time.Sleep(interval)
	}
}
