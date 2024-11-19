package backend

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"inference.networking.x-k8s.io/llm-instance-gateway/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdateDatastore_LLMServerPoolReconciler(t *testing.T) {
	tests := []struct {
		name               string
		datastore          K8sDatastore
		incomingServerPool *v1alpha1.LLMServerPool
		want               K8sDatastore
	}{
		{
			name: "Update to new, fresh LLMServerPool",
			datastore: K8sDatastore{
				LLMServerPool: &v1alpha1.LLMServerPool{
					Spec: v1alpha1.LLMServerPoolSpec{
						ModelServerSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "vllm"},
						},
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-pool",
						ResourceVersion: "Old and boring",
					},
				},
			},
			incomingServerPool: &v1alpha1.LLMServerPool{
				Spec: v1alpha1.LLMServerPoolSpec{
					ModelServerSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "not-vllm"},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-pool",
					ResourceVersion: "New and fun",
				},
			},
			want: K8sDatastore{
				LLMServerPool: &v1alpha1.LLMServerPool{
					Spec: v1alpha1.LLMServerPoolSpec{
						ModelServerSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "not-vllm"},
						},
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-pool",
						ResourceVersion: "New and fun",
					},
				},
			},
		},
		{
			name: "Do not update, resource version the same",
			datastore: K8sDatastore{
				LLMServerPool: &v1alpha1.LLMServerPool{
					Spec: v1alpha1.LLMServerPoolSpec{
						ModelServerSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "vllm"},
						},
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-pool",
						ResourceVersion: "Old and boring",
					},
				},
			},
			incomingServerPool: &v1alpha1.LLMServerPool{
				Spec: v1alpha1.LLMServerPoolSpec{
					ModelServerSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"technically": "this-should-never-happen"},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-pool",
					ResourceVersion: "Old and boring",
				},
			},
			want: K8sDatastore{
				LLMServerPool: &v1alpha1.LLMServerPool{
					Spec: v1alpha1.LLMServerPoolSpec{
						ModelServerSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "vllm"},
						},
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-pool",
						ResourceVersion: "Old and boring",
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			llmServerPoolReconciler := &LLMServerPoolReconciler{Datastore: &test.datastore}
			llmServerPoolReconciler.updateDatastore(test.incomingServerPool)

			if diff := cmp.Diff(test.want.LLMServerPool, llmServerPoolReconciler.Datastore.LLMServerPool); diff != "" {
				t.Errorf("Unexpected output (-want +got): %v", diff)
			}
		})
	}
}
