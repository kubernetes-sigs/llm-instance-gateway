package backend

import (
	"sync"
	"testing"

	"inference.networking.x-k8s.io/llm-instance-gateway/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	service1 = &v1alpha1.LLMService{
		Spec: v1alpha1.LLMServiceSpec{
			Models: []v1alpha1.Model{
				{
					Name: "fake model",
				},
			},
			PoolRef: []v1.ObjectReference{
				{
					Name: "test-pool",
					Kind: "llmserverpool",
				},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-service",
		},
	}
	service1Modified = &v1alpha1.LLMService{
		Spec: v1alpha1.LLMServiceSpec{
			Models: []v1alpha1.Model{
				{
					Name: "fake model",
				},
			},
			PoolRef: []v1.ObjectReference{
				{
					Name: "test-poolio",
					Kind: "llmserverpool",
				},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-service",
		},
	}
	service2 = &v1alpha1.LLMService{
		Spec: v1alpha1.LLMServiceSpec{
			Models: []v1alpha1.Model{
				{
					Name: "fake model",
				},
			},
			PoolRef: []v1.ObjectReference{
				{
					Name: "test-pool",
					Kind: "llmserverpool",
				},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-service-2",
		},
	}
)

func TestUpdateDatastore_LLMServiceReconciler(t *testing.T) {
	tests := []struct {
		name            string
		datastore       K8sDatastore
		incomingService *v1alpha1.LLMService
		want            K8sDatastore
	}{
		{
			name: "No Services registered; valid, new service incoming.",
			datastore: K8sDatastore{
				LLMServerPool: &v1alpha1.LLMServerPool{
					Spec: v1alpha1.LLMServerPoolSpec{
						ModelServerSelector: map[string]string{"app": "vllm"},
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-pool",
						ResourceVersion: "Old and boring",
					},
				},
				LLMServices: &sync.Map{},
			},
			incomingService: service1,
			want: K8sDatastore{
				LLMServerPool: &v1alpha1.LLMServerPool{
					Spec: v1alpha1.LLMServerPoolSpec{
						ModelServerSelector: map[string]string{"app": "not-vllm"},
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-pool",
						ResourceVersion: "New and fun",
					},
				},
				LLMServices: populateServiceMap(service1),
			},
		},
		{
			name: "Removing existing service.",
			datastore: K8sDatastore{
				LLMServerPool: &v1alpha1.LLMServerPool{
					Spec: v1alpha1.LLMServerPoolSpec{
						ModelServerSelector: map[string]string{"app": "vllm"},
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-pool",
						ResourceVersion: "Old and boring",
					},
				},
				LLMServices: populateServiceMap(service1),
			},
			incomingService: service1Modified,
			want: K8sDatastore{
				LLMServerPool: &v1alpha1.LLMServerPool{
					Spec: v1alpha1.LLMServerPoolSpec{
						ModelServerSelector: map[string]string{"app": "not-vllm"},
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-pool",
						ResourceVersion: "New and fun",
					},
				},
				LLMServices: populateServiceMap(),
			},
		},
		{
			name: "Unrelated service, do nothing.",
			datastore: K8sDatastore{
				LLMServerPool: &v1alpha1.LLMServerPool{
					Spec: v1alpha1.LLMServerPoolSpec{
						ModelServerSelector: map[string]string{"app": "vllm"},
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-pool",
						ResourceVersion: "Old and boring",
					},
				},
				LLMServices: populateServiceMap(service1),
			},
			incomingService: &v1alpha1.LLMService{
				Spec: v1alpha1.LLMServiceSpec{
					Models: []v1alpha1.Model{
						{
							Name: "fake model",
						},
					},
					PoolRef: []v1.ObjectReference{
						{
							Name: "test-poolio",
							Kind: "llmserverpool",
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "unrelated-service",
				},
			},
			want: K8sDatastore{
				LLMServerPool: &v1alpha1.LLMServerPool{
					Spec: v1alpha1.LLMServerPoolSpec{
						ModelServerSelector: map[string]string{"app": "not-vllm"},
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-pool",
						ResourceVersion: "New and fun",
					},
				},
				LLMServices: populateServiceMap(service1),
			},
		},
		{
			name: "Add to existing",
			datastore: K8sDatastore{
				LLMServerPool: &v1alpha1.LLMServerPool{
					Spec: v1alpha1.LLMServerPoolSpec{
						ModelServerSelector: map[string]string{"app": "vllm"},
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-pool",
						ResourceVersion: "Old and boring",
					},
				},
				LLMServices: populateServiceMap(service1),
			},
			incomingService: service2,
			want: K8sDatastore{
				LLMServerPool: &v1alpha1.LLMServerPool{
					Spec: v1alpha1.LLMServerPoolSpec{
						ModelServerSelector: map[string]string{"app": "not-vllm"},
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-pool",
						ResourceVersion: "New and fun",
					},
				},
				LLMServices: populateServiceMap(service1, service2),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			llmServiceReconciler := &LLMServiceReconciler{Datastore: &test.datastore, ServerPoolName: test.datastore.LLMServerPool.Name}
			llmServiceReconciler.updateDatastore(test.incomingService)

			if ok := mapsEqual(llmServiceReconciler.Datastore.LLMServices, test.want.LLMServices); !ok {
				t.Error("Maps are not equal")
			}
		})
	}
}

func populateServiceMap(services ...*v1alpha1.LLMService) *sync.Map {
	returnVal := &sync.Map{}

	for _, service := range services {
		returnVal.Store(service.Name, service)
	}
	return returnVal
}
