// Package scheduling implements request scheduling algorithms.
package scheduling

import (
	"math/rand"

	klog "k8s.io/klog/v2"

	"ext-proc/backend"
)

var (
	defaultFilter = &filter{
		name:   "least queuing",
		filter: leastQueuingFilterFunc,
		nextOnSuccessOrFailure: &filter{
			name:   "lora affinity",
			filter: toFilterFunc(loraAffinityPredicate),
			nextOnSuccessOrFailure: &filter{
				name:   "least KV cache percent",
				filter: leastKVCacheFilterFunc,
			},
		},
	}
)

func NewScheduler(pmp PodMetricsProvider) *Scheduler {
	return &Scheduler{
		podMetricsProvider: pmp,
		filter:             defaultFilter,
	}
}

type Scheduler struct {
	podMetricsProvider PodMetricsProvider
	filter             Filter
}

// PodMetricsProvider is an interface to provide set of pods in the backend and information such as
// metrics.
type PodMetricsProvider interface {
	AllPodMetrics() []*backend.PodMetrics
}

// Schedule finds the target pod based on metrics and the requested lora adapter.
func (s *Scheduler) Schedule(b *LLMRequest) (targetPod *backend.Pod, err error) {
	klog.V(2).Infof("request: %v; metrics: %+v", b, s.podMetricsProvider.AllPodMetrics())
	pods, err := s.filter.Filter(b, s.podMetricsProvider.AllPodMetrics())
	if err != nil || len(pods) == 0 {
		klog.Errorf("Failed to apply filter, this should never happen: %v", err)
	}
	i := rand.Intn(len(pods))
	return &pods[i].Pod, nil
}
