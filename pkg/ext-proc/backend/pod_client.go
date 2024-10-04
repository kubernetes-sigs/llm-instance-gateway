package backend

import (
	"fmt"
	"net/http"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	klog "k8s.io/klog/v2"
)

type PodMetricsClientImpl struct {
}

// FetchMetrics fetches metrics from a given pod.
func (p *PodMetricsClientImpl) FetchMetrics(pod Pod) (map[string]*dto.MetricFamily, error) {
	// Currently the metrics endpoint is hard-coded, which works with vLLM.
	// TODO(https://github.com/kubernetes-sigs/llm-instance-gateway/issues/16): Consume this from LLMServerPool config.
	url := fmt.Sprintf("http://%s/metrics", pod.Address)
	resp, err := http.Get(url)
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
	return parser.TextToMetricFamilies(resp.Body)
}
