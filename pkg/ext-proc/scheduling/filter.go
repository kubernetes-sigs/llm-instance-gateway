package scheduling

import (
	"fmt"
	"math"

	klog "k8s.io/klog/v2"

	"ext-proc/backend"
)

type Filter interface {
	Name() string
	Filter(b *LLMRequest, pods []*backend.PodMetrics) ([]*backend.PodMetrics, error)
}

// filter applies current filterFunc, and then recursively applies next filters depending success or
// failure of the current filterFunc.
// It can be used to construct a flow chart algorithm.
type filter struct {
	name   string
	filter filterFunc
	// nextOnSuccess filter will be applied after successfully applying the current filter.
	// The filtered results will be passed to the next filter.
	nextOnSuccess *filter
	// nextOnFailure filter will be applied if current filter fails.
	// The original input will be passed to the next filter.
	nextOnFailure *filter
	// nextOnSuccessOrFailure is a convenience field to configure the next filter regardless of the
	// success or failure of the current filter.
	// NOTE: When using nextOnSuccessOrFailure, both nextOnSuccess and nextOnFailure SHOULD be nil.
	// However if that's not the case, nextOnSuccess and nextOnFailure will be used, instead of
	// nextOnSuccessOrFailure,  in the success and failure scenarios, respectively.
	nextOnSuccessOrFailure *filter
}

func (f *filter) Name() string {
	if f == nil {
		return "nil"
	}
	return f.name
}

func (f *filter) Filter(b *LLMRequest, pods []*backend.PodMetrics) ([]*backend.PodMetrics, error) {
	if f == nil {
		klog.V(3).Infof("Running nil filter, returning all input pods by default")
		return pods, nil
	}
	klog.V(3).Infof("Running filter %q on request %v with %v pods", f.name, b, len(pods))

	filtered, err := f.filter(b, pods)

	next := f.nextOnSuccessOrFailure
	if err == nil {
		klog.V(3).Infof("onSuccess %v -> %v, filtered: %v", f.name, next.Name(), len(filtered))
		if f.nextOnSuccess != nil {
			next = f.nextOnSuccess
		}
		// On success, pass the filtered result to the next filter.
		return next.Filter(b, filtered)
	}

	klog.V(3).Infof("onFailure %v -> %v", f.name, next.Name())
	if f.nextOnFailure != nil {
		next = f.nextOnFailure
	}
	// On failure, pass the initial set of pods to the next filter.
	return next.Filter(b, pods)
}

// filterFunc filters a set of input pods to a subset.
type filterFunc func(b *LLMRequest, pods []*backend.PodMetrics) ([]*backend.PodMetrics, error)

// toFilterFunc is a helper function to convert a per pod filter func to the FilterFunc.
func toFilterFunc(pp podPredicate) filterFunc {
	return func(b *LLMRequest, pods []*backend.PodMetrics) ([]*backend.PodMetrics, error) {
		filtered := []*backend.PodMetrics{}
		for _, pod := range pods {
			pass := pp(b, pod)
			if pass {
				filtered = append(filtered, pod)
			}
		}
		if len(filtered) == 0 {
			return nil, fmt.Errorf("no pods left")
		}
		return filtered, nil
	}
}

// leastQueuingFilterFunc finds the max and min queue size of all pods, divides the whole range
// (max-min) by the number of pods, and finds the pods that fall into the first range.
// The intuition is that if there are multiple pods that share similar queue size in the low range,
// we should consider them all instead of the absolute minimum one. This worked better than picking
// the least one as it gives more choices for the next filter, which on aggregate gave better
// results.
// TODO: Compare this strategy with other strategies such as top K.
func leastQueuingFilterFunc(b *LLMRequest, pods []*backend.PodMetrics) ([]*backend.PodMetrics, error) {
	min := math.MaxInt
	max := 0
	filtered := []*backend.PodMetrics{}

	for _, pod := range pods {
		if pod.WaitingQueueSize <= min {
			min = pod.WaitingQueueSize
		}
		if pod.WaitingQueueSize >= max {
			max = pod.WaitingQueueSize
		}
	}

	for _, pod := range pods {
		if pod.WaitingQueueSize >= min && pod.WaitingQueueSize <= min+(max-min)/len(pods) {
			filtered = append(filtered, pod)
		}
	}
	return filtered, nil
}

// leastKVCacheFilterFunc finds the max and min KV cache of all pods, divides the whole range
// (max-min) by the number of pods, and finds the pods that fall into the first range.
// The intuition is that if there are multiple pods that share similar KV cache in the low range, we
// should consider them all instead of the absolute minimum one. This worked better than picking the
// least one as it gives more choices for the next filter, which on aggregate gave better results.
// TODO: Compare this strategy with other strategies such as top K.
func leastKVCacheFilterFunc(b *LLMRequest, pods []*backend.PodMetrics) ([]*backend.PodMetrics, error) {
	min := math.MaxFloat64
	max := math.SmallestNonzeroFloat64
	filtered := []*backend.PodMetrics{}

	for _, pod := range pods {
		if pod.KVCacheUsagePercent <= min {
			min = pod.KVCacheUsagePercent
		}
		if pod.KVCacheUsagePercent >= max {
			max = pod.KVCacheUsagePercent
		}
	}

	for _, pod := range pods {
		if pod.KVCacheUsagePercent >= min && pod.KVCacheUsagePercent <= min+(max-min)/float64(len(pods)) {
			filtered = append(filtered, pod)
		}
	}
	return filtered, nil
}

// podPredicate is a filter function to check whether a pod is desired.
type podPredicate func(b *LLMRequest, pod *backend.PodMetrics) bool

// loraAffinityPredicate return true if the pod have the requested LoRA adapter loaded.
func loraAffinityPredicate(b *LLMRequest, pod *backend.PodMetrics) bool {
	_, ok := pod.CachedModels[b.ResolvedTargetModel]
	return ok
}
