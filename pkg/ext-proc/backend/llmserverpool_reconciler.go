package backend

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
	"sigs.k8s.io/controller-runtime/pkg/client"

	// K8s imports
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	// I-GW imports
	"inference.networking.x-k8s.io/llm-instance-gateway/api/v1alpha1"
	clientset "inference.networking.x-k8s.io/llm-instance-gateway/client-go/clientset/versioned"
	informers "inference.networking.x-k8s.io/llm-instance-gateway/client-go/informers/externalversions/api/v1alpha1"
	listers "inference.networking.x-k8s.io/llm-instance-gateway/client-go/listers/api/v1alpha1"
)

const (
	reconcilerNamePrefix = "instance-gateway-"
)

// LLMServerPoolReconciler utilizes the controller runtime to reconcile Instance Gateway resources
// This implementation is just used for reading & maintaining data sync. The Gateway implementation
// will have the proper controller that will create/manage objects on behalf of the server pool.
type LLMServerPoolReconciler struct {
	client.Client
	//Scheme         *runtime.Scheme
	Record            record.EventRecorder
	ServerPoolName    string
	Namespace         string
	Datastore         *K8sDatastore
	Zone              string
	v1alpha1clientset clientset.Interface
	poolLister        listers.LLMServerPoolLister
	poolSynced        cache.InformerSynced

	workqueue workqueue.TypedRateLimitingInterface[cache.ObjectName]
	// Still uses recorder
}

func NewLLMServerPoolReconciler(
	//Params:
	ctx context.Context,
	serverPoolInformer informers.LLMServerPoolInformer,
	clientset clientset.Interface,
	kubeclientset kubernetes.Interface,
	scheme *runtime.Scheme,
	serverpoolName string,
	namespace string,
	zone string,
	datastore *K8sDatastore,
	//Return val:
) *LLMServerPoolReconciler {
	// Body:
	//utilruntime.Must(v1alpha1scheme.AddToScheme(scheme.Scheme))

	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster(record.WithContext(ctx))
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme, corev1.EventSource{Component: "FIGUREOUT NAMING HERE"})
	ratelimiter := workqueue.NewTypedMaxOfRateLimiter(
		workqueue.NewTypedItemExponentialFailureRateLimiter[cache.ObjectName](5*time.Millisecond, 1000*time.Second),
		&workqueue.TypedBucketRateLimiter[cache.ObjectName]{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	reconciler := &LLMServerPoolReconciler{
		v1alpha1clientset: clientset,
		poolLister:        serverPoolInformer.Lister(),
		poolSynced:        serverPoolInformer.Informer().HasSynced,
		Record:            recorder,
		workqueue:         workqueue.NewTypedRateLimitingQueue(ratelimiter),
		ServerPoolName:    serverpoolName,
		Namespace:         namespace,
		Zone:              zone,
		Datastore:         datastore,
	}
	klog.Info("Setting up event handlers")

	serverPoolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: reconciler.enqueuePool,
		UpdateFunc: func(old, new interface{}) {
			reconciler.enqueuePool(new)
		},
		DeleteFunc: reconciler.enqueuePool,
	})

	return reconciler
}

func (r *LLMServerPoolReconciler) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer r.workqueue.ShutDown()

	klog.Info("Starting Pool Reconciler")

	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(ctx.Done(), r.poolSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers ", "count ", workers)
	// Launch two workers to process Foo resources
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, r.runWorker, time.Second)
	}

	klog.Info("Started workers")
	<-ctx.Done()
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (r *LLMServerPoolReconciler) runWorker(ctx context.Context) {
	for r.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (r *LLMServerPoolReconciler) processNextWorkItem(ctx context.Context) bool {
	objRef, shutdown := r.workqueue.Get()
	logger := klog.FromContext(ctx)

	if shutdown {
		return false
	}

	// We call Done at the end of this func so the workqueue knows we have
	// finished processing this item. We also must remember to call Forget
	// if we do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the workqueue and attempted again after a back-off
	// period.
	defer r.workqueue.Done(objRef)

	// Run the syncHandler, passing it the structured reference to the object to be synced.
	klog.V(3).Infof("Reconciling Pool: %v", objRef.Name)
	err := r.reconcile(ctx, objRef)
	if err == nil {
		// If no error occurs then we Forget this item so it does not
		// get queued again until another change happens.
		r.workqueue.Forget(objRef)
		logger.Info("Successfully synced", "objectName", objRef)
		return true
	}

	utilruntime.HandleErrorWithContext(ctx, err, "Error syncing; requeuing for later retry", "objectReference", objRef)
	// since we failed, we should requeue the item to work on later.  This
	// method will add a backoff to avoid hotlooping on particular items
	r.workqueue.AddRateLimited(objRef)
	return true
}

func (r *LLMServerPoolReconciler) enqueuePool(obj interface{}) {
	if objectRef, err := cache.ObjectToName(obj); err != nil {
		utilruntime.HandleError(err)
	} else {
		r.workqueue.Add(objectRef)
	}
}

// Needs rework
func (r *LLMServerPoolReconciler) reconcile(ctx context.Context, objectRef cache.ObjectName) error {
	klog.Infof("Object name: %v, ObjectNamespace: %v, reconciler expected name: %v reconcilier namespace: %v", objectRef.Name, objectRef.Namespace, r.ServerPoolName, r.Namespace)
	if objectRef.Name != r.ServerPoolName && objectRef.Namespace != r.Namespace {
		return nil
	}
	klog.V(1).Info("reconciling LLMServerPool", objectRef.Name)

	serverPool, err := r.poolLister.LLMServerPools(objectRef.Namespace).Get(objectRef.Name)
	if err != nil {
		klog.Error(err, "unable to get LLMServerPool")
		return err
	}

	r.updateDatastore(serverPool)

	return nil
}

func (c *LLMServerPoolReconciler) updateDatastore(serverPool *v1alpha1.LLMServerPool) {
	if c.Datastore.LLMServerPool == nil || serverPool.ObjectMeta.ResourceVersion != c.Datastore.LLMServerPool.ObjectMeta.ResourceVersion {
		c.Datastore.LLMServerPool = serverPool
	}
}
