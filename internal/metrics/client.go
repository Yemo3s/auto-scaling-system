package metrics

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

// MetricsClient 定义了获取指标的接口
type MetricsClient interface {
	GetPodMetrics(namespace string) (*metricsv1beta1.PodMetricsList, error)
}

// K8sMetricsClient 实现 MetricsClient 接口
type K8sMetricsClient struct {
	client metrics.Interface
}

// NewK8sMetricsClient 创建新的 Kubernetes metrics 客户端
func NewK8sMetricsClient(client metrics.Interface) MetricsClient {
	return &K8sMetricsClient{
		client: client,
	}
}

// GetPodMetrics 获取指定命名空间的 Pod 指标
func (c *K8sMetricsClient) GetPodMetrics(namespace string) (*metricsv1beta1.PodMetricsList, error) {
	return c.client.MetricsV1beta1().PodMetricses(namespace).List(context.Background(), metav1.ListOptions{})
}
