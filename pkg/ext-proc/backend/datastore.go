package backend

import (
	"math/rand"
	"sync"

	"inference.networking.x-k8s.io/llm-instance-gateway/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// The datastore is a local cache of relevant data for the given LLMServerPool (currently all pulled from k8s-api)
type K8sDatastore struct {
	LLMServerPool *v1alpha1.LLMServerPool
	LLMServices   *sync.Map
	Pods          *sync.Map
}

func (ds *K8sDatastore) GetPodIPs() []string {
	var ips []string
	ds.Pods.Range(func(name, pod any) bool {
		ips = append(ips, pod.(*corev1.Pod).Status.PodIP)
		return true
	})
	return ips
}

func RandomWeightedDraw(model *v1alpha1.Model, seed int64) string {
	weights := 0

	source := rand.NewSource(rand.Int63())
	if seed > 0 {
		source = rand.NewSource(seed)
	}
	r := rand.New(source)
	for _, model := range model.TargetModels {
		weights += model.Weight
	}
	klog.Infof("Weights for Model(%v) total to: %v", model.Name, weights)
	randomVal := r.Intn(weights)
	for _, model := range model.TargetModels {
		if randomVal < model.Weight {
			return model.Name
		}
		randomVal -= model.Weight
	}
	return ""
}

func ModelHasObjective(model *v1alpha1.Model) bool {
	if model.Objective != nil && model.Objective.DesiredAveragePerOutputTokenLatencyAtP95OverMultipleRequests != nil {
		return true
	}

	return false
}
