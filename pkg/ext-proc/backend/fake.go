package backend

import (
	dto "github.com/prometheus/client_model/go"
)

type FakePodLister struct {
	Err  error
	Pods PodSet
}

type FakePodMetricsClient struct {
	Err map[Pod]error
	Res map[Pod]map[string]*dto.MetricFamily
}

func (f *FakePodMetricsClient) FetchMetrics(pod Pod) (map[string]*dto.MetricFamily, error) {
	if err, ok := f.Err[pod]; ok {
		return nil, err
	}
	return f.Res[pod], nil
}

func (fpl *FakePodLister) List() (PodSet, error) {
	return fpl.Pods, fpl.Err
}
