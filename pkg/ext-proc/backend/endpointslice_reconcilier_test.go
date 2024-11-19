package backend

import (
	"sync"
	"testing"

	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
)

var (
	basePod1 = Pod{Name: "pod1"}
	basePod2 = Pod{Name: "pod2"}
	basePod3 = Pod{Name: "pod3"}
)

func TestUpdateDatastore_EndpointSliceReconciler(t *testing.T) {
	tests := []struct {
		name          string
		datastore     K8sDatastore
		incomingSlice *discoveryv1.EndpointSlice
		want          K8sDatastore
	}{
		{
			name: "Add new pod",
			datastore: K8sDatastore{
				Pods: populateMap(basePod1, basePod2),
				Port: "8000",
			},
			incomingSlice: &discoveryv1.EndpointSlice{
				Endpoints: []discoveryv1.Endpoint{
					{
						TargetRef: &v1.ObjectReference{
							Name: "pod1",
						},
						Zone: new(string),
						Conditions: discoveryv1.EndpointConditions{
							Ready: truePointer(),
						},
						Addresses: []string{"0.0.0.0"},
					},
					{
						TargetRef: &v1.ObjectReference{
							Name: "pod2",
						},
						Zone: new(string),
						Conditions: discoveryv1.EndpointConditions{
							Ready: truePointer(),
						},
						Addresses: []string{"0.0.0.0"},
					},
					{
						TargetRef: &v1.ObjectReference{
							Name: "pod3",
						},
						Zone: new(string),
						Conditions: discoveryv1.EndpointConditions{
							Ready: truePointer(),
						},
						Addresses: []string{"0.0.0.0"},
					},
				},
			},
			want: K8sDatastore{
				Pods: populateMap(basePod1, basePod2, basePod3),
				Port: "8000",
			},
		},
		{
			name: "New pod, but its not ready yet. Do not add.",
			datastore: K8sDatastore{
				Pods: populateMap(basePod1, basePod2),
				Port: "8000",
			},
			incomingSlice: &discoveryv1.EndpointSlice{
				Endpoints: []discoveryv1.Endpoint{
					{
						TargetRef: &v1.ObjectReference{
							Name: "pod1",
						},
						Zone: new(string),
						Conditions: discoveryv1.EndpointConditions{
							Ready: truePointer(),
						},
						Addresses: []string{"0.0.0.0"},
					},
					{
						TargetRef: &v1.ObjectReference{
							Name: "pod2",
						},
						Zone: new(string),
						Conditions: discoveryv1.EndpointConditions{
							Ready: truePointer(),
						},
						Addresses: []string{"0.0.0.0"},
					},
					{
						TargetRef: &v1.ObjectReference{
							Name: "pod3",
						},
						Zone: new(string),
						Conditions: discoveryv1.EndpointConditions{
							Ready: new(bool),
						},
						Addresses: []string{"0.0.0.0"},
					},
				},
			},
			want: K8sDatastore{
				Pods: populateMap(basePod1, basePod2),
				Port: "8000",
			},
		},
		{
			name: "Existing pod not ready, new pod added, and is ready",
			datastore: K8sDatastore{
				Pods: populateMap(basePod1, basePod2),
				Port: "8000",
			},
			incomingSlice: &discoveryv1.EndpointSlice{
				Endpoints: []discoveryv1.Endpoint{
					{
						TargetRef: &v1.ObjectReference{
							Name: "pod1",
						},
						Zone: new(string),
						Conditions: discoveryv1.EndpointConditions{
							Ready: new(bool),
						},
						Addresses: []string{"0.0.0.0"},
					},
					{
						TargetRef: &v1.ObjectReference{
							Name: "pod2",
						},
						Zone: new(string),
						Conditions: discoveryv1.EndpointConditions{
							Ready: truePointer(),
						},
						Addresses: []string{"0.0.0.0"},
					},
					{
						TargetRef: &v1.ObjectReference{
							Name: "pod3",
						},
						Zone: new(string),
						Conditions: discoveryv1.EndpointConditions{
							Ready: truePointer(),
						},
						Addresses: []string{"0.0.0.0"},
					},
				},
			},
			want: K8sDatastore{
				Pods: populateMap(basePod3, basePod2),
				Port: "8000",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			endpointSliceReconciler := &EndpointSliceReconciler{Datastore: &test.datastore, Zone: ""}
			endpointSliceReconciler.updateDatastore(test.incomingSlice)

			if mapsEqual(endpointSliceReconciler.Datastore.Pods, test.want.Pods) {
				t.Errorf("Unexpected output pod mismatch. \n Got %v \n Want: %v \n", endpointSliceReconciler.Datastore.Pods, test.want.Pods)
			}
		})
	}
}

func mapsEqual(map1, map2 *sync.Map) bool {
	equal := true

	map1.Range(func(k, v any) bool {
		if _, ok := map2.Load(k); !ok {
			equal = false
			return false
		}
		return true
	})
	map2.Range(func(k, v any) bool {
		if _, ok := map1.Load(k); !ok {
			equal = false
			return false
		}
		return true
	})

	return equal
}

func truePointer() *bool {
	primitivePointersAreSilly := true
	return &primitivePointersAreSilly
}
