package vllm

import (
	"testing"

	"inference.networking.x-k8s.io/llm-instance-gateway/pkg/ext-proc/backend"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestPromToPodMetrics(t *testing.T) {
	testCases := []struct {
		name              string
		metricFamilies    map[string]*dto.MetricFamily
		expectedMetrics   *backend.Metrics
		expectedErr       error
		initialPodMetrics *backend.PodMetrics
	}{
		{
			name: "all metrics available",
			metricFamilies: map[string]*dto.MetricFamily{
				RunningQueueSizeMetricName: {
					Metric: []*dto.Metric{
						{
							Gauge: &dto.Gauge{
								Value: proto.Float64(10),
							},
							TimestampMs: proto.Int64(100),
						},
						{
							Gauge: &dto.Gauge{
								Value: proto.Float64(15),
							},
							TimestampMs: proto.Int64(200), // This is the latest
						},
					},
				},
				WaitingQueueSizeMetricName: {
					Metric: []*dto.Metric{
						{
							Gauge: &dto.Gauge{
								Value: proto.Float64(20),
							},
							TimestampMs: proto.Int64(100),
						},
						{
							Gauge: &dto.Gauge{
								Value: proto.Float64(25),
							},
							TimestampMs: proto.Int64(200), // This is the latest
						},
					},
				},
				KVCacheUsagePercentMetricName: {
					Metric: []*dto.Metric{
						{
							Gauge: &dto.Gauge{
								Value: proto.Float64(0.8),
							},
							TimestampMs: proto.Int64(100),
						},
						{
							Gauge: &dto.Gauge{
								Value: proto.Float64(0.9),
							},
							TimestampMs: proto.Int64(200), // This is the latest
						},
					},
				},
				ActiveLoRAAdaptersMetricName: {
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("active_adapters"),
									Value: proto.String("lora1,lora2"),
								},
							},
							Gauge: &dto.Gauge{
								Value: proto.Float64(100),
							},
						},
					},
				},
				LoraRequestInfoMetricName: {
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("running_lora_adapters"),
									Value: proto.String("lora3,lora4"),
								},
								{
									Name:  proto.String("waiting_lora_adapters"),
									Value: proto.String("lora1,lora4"),
								},
							},
							Gauge: &dto.Gauge{
								Value: proto.Float64(100),
							},
						},
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("running_lora_adapters"),
									Value: proto.String("lora2"),
								},
							},
							Gauge: &dto.Gauge{
								Value: proto.Float64(90),
							},
						},
					},
				},
			},
			expectedMetrics: &backend.Metrics{
				RunningQueueSize:    15,
				WaitingQueueSize:    25,
				KVCacheUsagePercent: 0.9,
				CachedModels: map[string]int{
					"lora3": 0,
					"lora4": 0,
				},
			},
			initialPodMetrics: &backend.PodMetrics{},
			expectedErr:       nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			updated, err := promToPodMetrics(tc.metricFamilies, tc.initialPodMetrics)
			if tc.expectedErr != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedMetrics, &updated.Metrics)
			}
		})
	}
}
