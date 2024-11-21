package backend

import (
	"sync"

	"inference.networking.x-k8s.io/llm-instance-gateway/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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
