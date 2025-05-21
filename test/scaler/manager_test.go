package scaler_test

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"testing"
	autoscalingv1 "yemo.info/auto-scaling-system/api/v1"
	"yemo.info/auto-scaling-system/internal/scaler"
)

// MockMetricsClient 模拟指标客户端
type MockMetricsClient struct {
	mock.Mock
}

func (m *MockMetricsClient) GetPodMetrics(namespace string) (*metricsv1beta1.PodMetricsList, error) {
	args := m.Called(namespace)
	return args.Get(0).(*metricsv1beta1.PodMetricsList), args.Error(1)
}

// 创建测试用的 HPAModifier
func createTestHPAModifier() *autoscalingv1.HPAModifier {
	return &autoscalingv1.HPAModifier{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hpa",
			Namespace: "default",
		},
		Spec: autoscalingv1.HPAModifierSpec{
			TargetRef: corev1.ObjectReference{
				Name:      "nginx-deployment",
				Namespace: "default",
			},
			MinReplicas:      1,
			MaxReplicas:      10,
			CPUThreshold:     0.7,
			MemoryThreshold:  0.8,
			PredictionWindow: 300,
		},
		Status: autoscalingv1.HPAModifierStatus{
			CurrentReplicas: 1,
		},
	}
}

// 创建测试用的 Pod 指标数据
func createTestPodMetrics() *metricsv1beta1.PodMetricsList {
	return &metricsv1beta1.PodMetricsList{
		Items: []metricsv1beta1.PodMetrics{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-deployment-9d9b49c9b-64sbk",
					Namespace: "default",
					Labels: map[string]string{
						"app": "nginx",
					},
				},
				Containers: []metricsv1beta1.ContainerMetrics{
					{
						Name: "nginx",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
		},
	}
}

func TestCollectMetrics(t *testing.T) {
	// 创建模拟的客户端
	fakeKubeClient := fake.NewSimpleClientset()
	mockMetricsClient := &MockMetricsClient{}

	// 创建测试数据
	podMetrics := createTestPodMetrics()

	// 设置模拟行为
	mockMetricsClient.On("GetPodMetrics", "default").Return(podMetrics, nil)

	// 创建伸缩管理器，使用自定义的 mock metrics client
	manager := &scaler.ScalingManager{
		KubeClient:    fakeKubeClient,
		MetricsClient: mockMetricsClient,
	}

	// 创建测试 HPAModifier
	hpa := createTestHPAModifier()

	// 测试收集指标
	cpuUsage, memoryUsage, err := manager.CollectMetrics(context.Background(), hpa)
	assert.NoError(t, err)
	assert.True(t, cpuUsage > 0)
	assert.True(t, memoryUsage > 0)

	// 打印详细的资源使用信息
	t.Logf("\n资源使用情况:")
	t.Logf("----------------------------------------")
	t.Logf("Pod名称: %s", podMetrics.Items[0].Name)
	t.Logf("命名空间: %s", podMetrics.Items[0].Namespace)
	t.Logf("标签: %v", podMetrics.Items[0].Labels)
	t.Logf("----------------------------------------")
	t.Logf("容器资源使用详情:")

	for _, container := range podMetrics.Items[0].Containers {
		cpuMilliValue := container.Usage.Cpu().MilliValue()
		cpuValue := float64(cpuMilliValue) / 1000.0
		memoryBytes := container.Usage.Memory().Value()
		memoryMB := float64(memoryBytes) / (1024 * 1024)

		t.Logf("  容器名称: %s", container.Name)
		t.Logf("  CPU使用: %.6f核 (%dm)", cpuValue, cpuMilliValue)
		t.Logf("  内存使用: %.2fMB (%d字节)", memoryMB, memoryBytes)
		t.Logf("----------------------------------------")
	}

	t.Logf("平均资源使用:")
	t.Logf("  CPU: %.6f核", cpuUsage)
	t.Logf("  内存: %.2fGB", memoryUsage)
	t.Logf("----------------------------------------")

	// 验证模拟方法被调用
	mockMetricsClient.AssertExpectations(t)
}

func TestCalculateDesiredReplicas(t *testing.T) {

}

func TestScaleWorkload(t *testing.T) {

}
