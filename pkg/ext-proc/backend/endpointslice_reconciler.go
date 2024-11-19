package backend

import (
	"context"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	serviceOwnerLabel = "kubernetes.io/service-name"
)

type EndpointSliceReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Record         record.EventRecorder
	ServerPoolName string
	ServiceName    string
	Zone           string
	Namespace      string
	Datastore      *K8sDatastore
}

func (c *EndpointSliceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.V(1).Info("reconciling EndpointSlice ", req.NamespacedName)

	endpointSlice := &discoveryv1.EndpointSlice{}
	if err := c.Get(ctx, req.NamespacedName, endpointSlice); err != nil {
		klog.Error(err, "unable to get LLMServerPool")
		return ctrl.Result{}, err
	}

	if !c.ownsEndPointSlice(endpointSlice.ObjectMeta.Labels) {
		return ctrl.Result{}, nil
	}

	c.updateDatastore(endpointSlice)

	return ctrl.Result{}, nil
}

func (c *EndpointSliceReconciler) updateDatastore(slice *discoveryv1.EndpointSlice) {
	podMap := make(map[Pod]bool)
	for _, endpoint := range slice.Endpoints {
		klog.V(4).Infof("Zone: %v \n endpoint: %+v \n", c.Zone, endpoint)
		if c.validPod(endpoint) {
			pod := Pod{Name: *&endpoint.TargetRef.Name, Address: endpoint.Addresses[0] + ":" + c.Datastore.Port}
			podMap[pod] = true
			c.Datastore.Pods.Store(pod, true)
		}
	}

	removeOldPods := func(k, v any) bool {
		pod, ok := k.(Pod)
		if !ok {
			klog.Errorf("Unable to cast key to Pod: %v", k)
			return false
		}
		if _, ok := podMap[pod]; !ok {
			c.Datastore.Pods.Delete(pod)
		}
		return true
	}
	c.Datastore.Pods.Range(removeOldPods)
}

func (c *EndpointSliceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1.EndpointSlice{}).
		Complete(c)
}

func (c *EndpointSliceReconciler) ownsEndPointSlice(labels map[string]string) bool {
	return labels[serviceOwnerLabel] == c.ServiceName
}

func (c *EndpointSliceReconciler) validPod(endpoint discoveryv1.Endpoint) bool {
	validZone := c.Zone == "" || c.Zone != "" && *endpoint.Zone == c.Zone
	return validZone && *endpoint.Conditions.Ready == true

}
