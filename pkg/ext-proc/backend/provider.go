package backend

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/multierr"
	klog "k8s.io/klog/v2"
)

func NewProvider(pmc PodMetricsClient, pl PodLister) *Provider {
	p := &Provider{
		podMetrics: sync.Map{},
		pmc:        pmc,
		pl:         pl,
	}
	return p
}

// Provider provides backend pods and information such as metrics.
type Provider struct {
	// key: Pod, value: *PodMetrics
	podMetrics sync.Map
	pmc        PodMetricsClient
	pl         PodLister
}

type PodMetricsClient interface {
	FetchMetrics(pod Pod, existing *PodMetrics) (*PodMetrics, error)
}

type PodLister interface {
	List() (PodSet, error)
}

func (p *Provider) AllPodMetrics() []*PodMetrics {
	res := []*PodMetrics{}
	fn := func(k, v any) bool {
		res = append(res, v.(*PodMetrics))
		return true
	}
	p.podMetrics.Range(fn)
	return res
}

func (p *Provider) UpdatePodMetrics(pod Pod, pm *PodMetrics) {
	p.podMetrics.Store(pod, pm)
}

func (p *Provider) GetPodMetrics(pod Pod) (*PodMetrics, bool) {
	val, ok := p.podMetrics.Load(pod)
	if ok {
		return val.(*PodMetrics), true
	}
	return nil, false
}

func (p *Provider) Init(refreshPodsInterval, refreshMetricsInterval time.Duration) error {
	if err := p.refreshPodsOnce(); err != nil {
		return fmt.Errorf("failed to init pods: %v", err)
	}
	if err := p.refreshMetricsOnce(); err != nil {
		return fmt.Errorf("failed to init metrics: %v", err)
	}

	klog.Infof("Initialized pods and metrics: %+v", p.AllPodMetrics())

	// periodically refresh pods
	go func() {
		for {
			time.Sleep(refreshPodsInterval)
			if err := p.refreshPodsOnce(); err != nil {
				klog.V(4).Infof("Failed to refresh podslist pods: %v", err)
			}
		}
	}()

	// periodically refresh metrics
	go func() {
		for {
			time.Sleep(refreshMetricsInterval)
			if err := p.refreshMetricsOnce(); err != nil {
				klog.V(4).Infof("Failed to refresh metrics: %v", err)
			}
		}
	}()

	// Periodically print out the pods and metrics for DEBUGGING.
	if klog.V(2).Enabled() {
		go func() {
			for {
				time.Sleep(5 * time.Second)
				klog.Infof("===DEBUG: Current Pods and metrics: %+v", p.AllPodMetrics())
			}
		}()
	}

	return nil
}

// refreshPodsOnce lists pods and updates keys in the podMetrics map.
// Note this function doesn't update the PodMetrics value, it's done separately.
func (p *Provider) refreshPodsOnce() error {
	pods, err := p.pl.List()
	if err != nil {
		return err
	}
	// merge new pods with cached ones.
	// add new pod to the map
	for pod := range pods {
		if _, ok := p.podMetrics.Load(pod); !ok {
			new := &PodMetrics{
				Pod: pod,
				Metrics: Metrics{
					ActiveModels: make(map[string]int),
				},
			}
			p.podMetrics.Store(pod, new)
		}
	}
	// remove pods that don't exist any more.
	mergeFn := func(k, v any) bool {
		pod := k.(Pod)
		if _, ok := pods[pod]; !ok {
			p.podMetrics.Delete(pod)
		}
		return true
	}
	p.podMetrics.Range(mergeFn)
	return nil
}

func (p *Provider) refreshMetricsOnce() error {
	start := time.Now()
	defer func() {
		d := time.Since(start)
		// TODO: add a metric instead of logging
		klog.V(4).Infof("Refreshed metrics in %v", d)
	}()
	var wg sync.WaitGroup
	var errs error
	processOnePod := func(key, value any) bool {
		klog.V(4).Infof("Processing pod %v and metric %v", key, value)
		pod := key.(Pod)
		existing := value.(*PodMetrics)
		wg.Add(1)
		go func() {
			defer wg.Done()
			updated, err := p.pmc.FetchMetrics(pod, existing)
			if err != nil {
				multierr.Append(errs, fmt.Errorf("failed to parse metrics from %s: %v", pod, err))
				return
			}
			klog.V(4).Infof("Updated metrics for pod %s: %v", pod, updated.Metrics)
			if err != nil {
				multierr.Append(errs, fmt.Errorf("failed to get all pod metrics updated from prometheus: %v", err))
			}
			p.UpdatePodMetrics(pod, updated)
		}()
		return true
	}
	p.podMetrics.Range(processOnePod)
	wg.Wait()
	return errs
}
