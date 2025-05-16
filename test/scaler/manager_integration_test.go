package scaler_test

import (
	"context"
	"testing"
	"time"

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

func TestCollectMetricsWithRealCluster(t *testing.T) {
	// 1. 创建真实的客户端连接
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename())
	assert.NoError(t, err, "应能加载 kubeconfig")

	// 创建 Kubernetes 客户端
	kubeClient, err := kubernetes.NewForConfig(config)
	assert.NoError(t, err, "应能创建 Kubernetes 客户端")

	// 创建 Metrics 客户端
	metricsClient, err := metrics.NewForConfig(config)
	assert.NoError(t, err, "应能创建 Metrics 客户端")

	// 2. 创建真实的 metrics client
	realMetricsClient := metrics2.NewK8sMetricsClient(metricsClient)

	// 3. 创建 ScalingManager
	manager := &scaler.ScalingManager{
		KubeClient:    kubeClient,
		MetricsClient: realMetricsClient,
	}

	// 4. 创建 HPAModifier 配置
	hpa := &autoscalingv1.HPAModifier{
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
	}

	// 5. 收集并展示实时指标
	t.Log("\n开始收集实时指标...")
	t.Log("----------------------------------------")

	// 收集3次数据，每次间隔2秒
	for i := 0; i < 3; i++ {
		cpuUsage, memoryUsage, err := manager.CollectMetrics(context.Background(), hpa)
		if err != nil {
			t.Logf("第 %d 次收集指标失败: %v", i+1, err)
			continue
		}

		// 获取 Pod 详细信息
		pods, err := kubeClient.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=nginx",
		})
		assert.NoError(t, err, "应能获取 Pod 列表")

		// 获取指标详情
		podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses("default").List(context.Background(), metav1.ListOptions{})
		assert.NoError(t, err, "应能获取 Pod 指标")

		t.Logf("\n第 %d 次采集 (时间: %s):", i+1, time.Now().Format("15:04:05"))
		t.Log("----------------------------------------")

		// 打印 Pod 信息
		for _, pod := range pods.Items {
			t.Logf("Pod名称: %s", pod.Name)
			t.Logf("状态: %s", pod.Status.Phase)
			t.Logf("节点: %s", pod.Spec.NodeName)

			// 查找该 Pod 的指标数据
			for _, metric := range podMetrics.Items {
				if metric.Name == pod.Name {
					for _, container := range metric.Containers {
						cpuMilliValue := container.Usage.Cpu().MilliValue()
						cpuValue := float64(cpuMilliValue) / 1000.0
						memoryBytes := container.Usage.Memory().Value()
						memoryMB := float64(memoryBytes) / (1024 * 1024)

						t.Logf("容器名称: %s", container.Name)
						t.Logf("  CPU使用: %.6f核 (%dm)", cpuValue, cpuMilliValue)
						t.Logf("  内存使用: %.2fMB (%d字节)", memoryMB, memoryBytes)
					}
				}
			}
		}

		t.Log("----------------------------------------")
		t.Logf("平均资源使用:")
		t.Logf("  CPU: %.6f核", cpuUsage)
		t.Logf("  内存: %.2fMB", memoryUsage*1024)
		t.Log("----------------------------------------")

		// 等待2秒后进行下一次采集
		if i < 2 {
			time.Sleep(2 * time.Second)
		}
	}
}
