package backend

import (
	"fmt"
	"math/rand"
	"sync"

	"inference.networking.x-k8s.io/llm-instance-gateway/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func NewK8sDataStore(options ...K8sDatastoreOption) *K8sDatastore {
	store := &K8sDatastore{
		poolMu:      sync.RWMutex{},
		llmServices: &sync.Map{},
		pods:        &sync.Map{},
	}
	for _, opt := range options {
		opt(store)
	}
	return store
}

// The datastore is a local cache of relevant data for the given LLMServerPool (currently all pulled from k8s-api)
type K8sDatastore struct {
	// poolMu is used to synchronize access to the llmServerPool.
	poolMu        sync.RWMutex
	llmServerPool *v1alpha1.LLMServerPool
	llmServices   *sync.Map
	pods          *sync.Map
}

type K8sDatastoreOption func(*K8sDatastore)

// WithPods can be used in tests to override the pods.
func WithPods(pods []*PodMetrics) K8sDatastoreOption {
	return func(store *K8sDatastore) {
		store.pods = &sync.Map{}
		for _, pod := range pods {
			store.pods.Store(pod.Pod, true)
		}
	}
}

func (ds *K8sDatastore) setLLMServerPool(pool *v1alpha1.LLMServerPool) {
	ds.poolMu.Lock()
	defer ds.poolMu.Unlock()
	ds.llmServerPool = pool
}

func (ds *K8sDatastore) getLLMServerPool() (*v1alpha1.LLMServerPool, error) {
	ds.poolMu.RLock()
	defer ds.poolMu.RUnlock()
	if ds.llmServerPool == nil {
		return nil, fmt.Errorf("LLMServerPool hasn't been initialized yet")
	}
	return ds.llmServerPool, nil
}

func (ds *K8sDatastore) GetPodIPs() []string {
	var ips []string
	ds.pods.Range(func(name, pod any) bool {
		ips = append(ips, pod.(*corev1.Pod).Status.PodIP)
		return true
	})
	return ips
}

func (s *K8sDatastore) FetchModelData(modelName string) (returnModel *v1alpha1.Model) {
	s.llmServices.Range(func(k, v any) bool {
		service := v.(*v1alpha1.LLMService)
		klog.V(3).Infof("Service name: %v", service.Name)
		for _, model := range service.Spec.Models {
			if model.Name == modelName {
				returnModel = &model
				// We want to stop iterating, return false.
				return false
			}
		}
		return true
	})
	return
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
	klog.V(3).Infof("Weights for Model(%v) total to: %v", model.Name, weights)
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
