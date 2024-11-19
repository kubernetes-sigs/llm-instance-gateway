package backend

import (
	"context"
	"fmt"
)

type FakePodMetricsClient struct {
	Err map[Pod]error
	Res map[Pod]*PodMetrics
}

func (f *FakePodMetricsClient) FetchMetrics(ctx context.Context, pod Pod, existing *PodMetrics) (*PodMetrics, error) {
	if err, ok := f.Err[pod]; ok {
		return nil, err
	}
	fmt.Printf("pod: %+v\n existing: %+v \n new: %+v \n", pod, existing, f.Res[pod])
	return f.Res[pod], nil
}
