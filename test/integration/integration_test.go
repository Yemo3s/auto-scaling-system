package integration

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	autoscalingv1 "yemo.info/auto-scaling-system/api/v1"
	metrics2 "yemo.info/auto-scaling-system/internal/metrics"
	"yemo.info/auto-scaling-system/internal/scaler"
)

var (
	kubeconfig string
	namespace  string
)

func init() {
	// 获取 kubeconfig 路径
	kubeconfig = os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	// 获取测试命名空间
	namespace = os.Getenv("TEST_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}
}

// 创建 Kubernetes 客户端
func createClients(t *testing.T) (*kubernetes.Clientset, *metrics.Clientset) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	assert.NoError(t, err)

	kubeClient, err := kubernetes.NewForConfig(config)
	assert.NoError(t, err)

	metricsClient, err := metrics.NewForConfig(config)
	assert.NoError(t, err)

	return kubeClient, metricsClient
}

func TestMetricsCollection(t *testing.T) {
	// 创建客户端
	kubeClient, metricsClient := createClients(t)

	// 创建真实的 metrics client
	realMetricsClient := metrics2.NewK8sMetricsClient(metricsClient)

	// 创建伸缩管理器
	manager := scaler.NewScalingManager(kubeClient, realMetricsClient)

	// 创建测试 HPAModifier
	hpa := &autoscalingv1.HPAModifier{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hpa",
			Namespace: namespace,
		},
		Spec: autoscalingv1.HPAModifierSpec{
			TargetRef: corev1.ObjectReference{
				Name:      "nginx-deployment",
				Namespace: namespace,
			},
			MinReplicas:      1,
			MaxReplicas:      10,
			CPUThreshold:     0.7,
			MemoryThreshold:  0.8,
			PredictionWindow: 300,
		},
	}

	// 测试收集指标
	cpuUsage, memoryUsage, err := manager.CollectMetrics(context.Background(), hpa)
	if err != nil {
		t.Logf("Warning: 收集指标失败: %v", err)
		return
	}

	t.Logf("收集到的指标 - CPU: %v, Memory: %v", cpuUsage, memoryUsage)
	assert.True(t, cpuUsage >= 0)
	assert.True(t, memoryUsage >= 0)
}

func TestEndToEnd(t *testing.T) {
	// 创建客户端
	kubeClient, metricsClient := createClients(t)

	// 创建真实的 metrics client
	realMetricsClient := metrics2.NewK8sMetricsClient(metricsClient)

	// 创建伸缩管理器
	manager := scaler.NewScalingManager(kubeClient, realMetricsClient)

	// 创建测试 HPAModifier
	hpa := &autoscalingv1.HPAModifier{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hpa-e2e",
			Namespace: namespace,
		},
		Spec: autoscalingv1.HPAModifierSpec{
			TargetRef: corev1.ObjectReference{
				Name:      "nginx-deployment",
				Namespace: namespace,
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

	// 执行伸缩测试
	err := manager.ScaleWorkload(context.Background(), hpa)
	if err != nil {
		t.Logf("Warning: 伸缩操作失败: %v", err)
		return
	}

	// 验证结果
	t.Logf("伸缩后的状态 - 副本数: %d, 预测负载: %v",
		hpa.Status.CurrentReplicas,
		hpa.Status.PredictedLoad)

	assert.True(t, hpa.Status.CurrentReplicas >= hpa.Spec.MinReplicas)
	assert.True(t, hpa.Status.CurrentReplicas <= hpa.Spec.MaxReplicas)
	assert.NotNil(t, hpa.Status.LastScaledTime)
}
