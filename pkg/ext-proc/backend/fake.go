package backend

import "context"

type FakePodLister struct {
	Err  error
	Pods PodSet
}

type FakePodMetricsClient struct {
	Err map[Pod]error
	Res map[Pod]*PodMetrics
}

func (f *FakePodMetricsClient) FetchMetrics(ctx context.Context, pod Pod, existing *PodMetrics) (*PodMetrics, error) {
	if err, ok := f.Err[pod]; ok {
		return nil, err
	}
	return f.Res[pod], nil
}

func (fpl *FakePodLister) List() (PodSet, error) {
	return fpl.Pods, fpl.Err
}
