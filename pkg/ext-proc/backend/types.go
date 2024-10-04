// Package backend is a library to interact with backend model servers such as probing metrics.
package backend

import "fmt"

type PodSet map[Pod]bool

type Pod struct {
	Namespace string
	Name      string
	Address   string
}

func (p Pod) String() string {
	return p.Namespace + "." + p.Name
}

type Metrics struct {
	// CachedModels is a set of models(including LoRA adapters) that are currently cached to GPU.
	CachedModels            map[string]int
	RunningQueueSize        int
	WaitingQueueSize        int
	KVCacheUsagePercent     float64
	KvCacheMaxTokenCapacity int
}

type PodMetrics struct {
	Pod
	Metrics
}

func (pm *PodMetrics) String() string {
	return fmt.Sprintf("Pod: %+v; Metrics: %+v", pm.Pod, pm.Metrics)
}

func (pm *PodMetrics) Clone() *PodMetrics {
	cm := make(map[string]int, len(pm.CachedModels))
	for k, v := range pm.CachedModels {
		cm[k] = v
	}
	clone := &PodMetrics{
		Pod: pm.Pod,
		Metrics: Metrics{
			CachedModels:            cm,
			RunningQueueSize:        pm.RunningQueueSize,
			WaitingQueueSize:        pm.WaitingQueueSize,
			KVCacheUsagePercent:     pm.KVCacheUsagePercent,
			KvCacheMaxTokenCapacity: pm.KvCacheMaxTokenCapacity,
		},
	}
	return clone
}
