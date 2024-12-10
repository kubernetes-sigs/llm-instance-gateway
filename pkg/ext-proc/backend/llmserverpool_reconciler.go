package backend

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"inference.networking.x-k8s.io/llm-instance-gateway/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	reconcilerNamePrefix = "instance-gateway-"
)

// LLMServerPoolReconciler utilizes the controller runtime to reconcile Instance Gateway resources
// This implementation is just used for reading & maintaining data sync. The Gateway implementation
// will have the proper controller that will create/manage objects on behalf of the server pool.
type LLMServerPoolReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Record         record.EventRecorder
	ServerPoolName string
	Namespace      string
	Datastore      *K8sDatastore
	Zone           string
}

func (c *LLMServerPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.NamespacedName.Name != c.ServerPoolName && req.NamespacedName.Namespace != c.Namespace {
		return ctrl.Result{}, nil
	}
	klog.V(2).Infof("Reconciling LLMServerPool %v", req.NamespacedName)

	serverPool := &v1alpha1.LLMServerPool{}
	if err := c.Get(ctx, req.NamespacedName, serverPool); err != nil {
		klog.Errorf("Unable to get LLMServerPool: %v", err)
		return ctrl.Result{}, err
	}

	klog.V(2).Infof("Updated LLMServerPool: %+v", serverPool)
	c.Datastore.setLLMServerPool(serverPool)

	return ctrl.Result{}, nil
}

func (c *LLMServerPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LLMServerPool{}).
		Complete(c)
}
