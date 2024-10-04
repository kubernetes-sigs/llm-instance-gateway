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
		klog.V(2).Infof("Running nil filter, returning all input pods by default")
		return pods, nil
	}
	klog.V(2).Infof("Running filter %q on request %v with %v pods", f.name, b, len(pods))

	filtered, err := f.filter(b, pods)

	next := f.nextOnSuccessOrFailure
	if err == nil {
		klog.V(2).Infof("onSuccess %v -> %v, filtered: %v", f.name, next.Name(), len(filtered))
		if f.nextOnSuccess != nil {
			next = f.nextOnSuccess
		}
		// On success, pass the filtered result to the next filter.
		return next.Filter(b, filtered)
	}

	klog.V(2).Infof("onFailure %v -> %v", f.name, next.Name())
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

func leastQueuingFilterFunc(b *LLMRequest, pods []*backend.PodMetrics) ([]*backend.PodMetrics, error) {
	min := math.MaxInt
	filtered := []*backend.PodMetrics{}
	for _, pod := range pods {
		if pod.WaitingQueueSize < min {
			min = pod.WaitingQueueSize
			filtered = []*backend.PodMetrics{}
		}
		if pod.WaitingQueueSize == min {
			filtered = append(filtered, pod)
		}
	}
	return filtered, nil
}

func leastKVCacheFilterFunc(b *LLMRequest, pods []*backend.PodMetrics) ([]*backend.PodMetrics, error) {
	min := math.MaxInt
	filtered := []*backend.PodMetrics{}
	margin := 5
	for _, pod := range pods {
		cur := int(pod.KVCacheUsagePercent) / margin
		if cur < min {
			min = cur
			filtered = []*backend.PodMetrics{}
		}
		if cur == min {
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
