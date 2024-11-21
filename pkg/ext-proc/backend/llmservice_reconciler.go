package backend

import (
	"context"
	"strings"

	"inference.networking.x-k8s.io/llm-instance-gateway/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LLMServiceReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Record         record.EventRecorder
	Datastore      *K8sDatastore
	ServerPoolName string
	Namespace      string
}

func (c *LLMServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Namespace != c.Namespace {
		return ctrl.Result{}, nil
	}
	klog.V(1).Info("reconciling LLMService", req.NamespacedName)

	service := &v1alpha1.LLMService{}
	if err := c.Get(ctx, req.NamespacedName, service); err != nil {
		klog.Error(err, "unable to get LLMServerPool")
		return ctrl.Result{}, err
	}

	c.updateDatastore(service)
	return ctrl.Result{}, nil
}

func (c *LLMServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LLMService{}).
		Complete(c)
}

func (c *LLMServiceReconciler) updateDatastore(service *v1alpha1.LLMService) {
	for _, ref := range service.Spec.PoolRef {
		if strings.Contains(strings.ToLower(ref.Kind), strings.ToLower("LLMServerPool")) && ref.Name == c.ServerPoolName {
			klog.V(2).Infof("Adding/Updating service: %v", service.Name)
			c.Datastore.LLMServices.Store(service.Name, service)
			return
		}
	}
	klog.V(2).Infof("Removing/Not adding service: %v", service.Name)
	// If we get here. The service is not relevant to this pool, remove.
	c.Datastore.LLMServices.Delete(service.Name)
}
