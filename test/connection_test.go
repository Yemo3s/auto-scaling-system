package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

func TestClusterConnection(t *testing.T) {
	// 1. 测试获取 kubeconfig
	kubeconfig := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	assert.NotEmpty(t, kubeconfig, "kubeconfig 路径不应为空")

	// 2. 测试创建 config
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	assert.NoError(t, err, "应能成功创建 config")
	assert.NotNil(t, config, "config 不应为 nil")

	// 3. 测试创建 kubernetes 客户端
	kubeClient, err := kubernetes.NewForConfig(config)
	assert.NoError(t, err, "应能成功创建 kubernetes 客户端")
	assert.NotNil(t, kubeClient, "kubernetes 客户端不应为 nil")

	// 4. 测试创建 metrics 客户端
	metricsClient, err := metrics.NewForConfig(config)
	assert.NoError(t, err, "应能成功创建 metrics 客户端")
	assert.NotNil(t, metricsClient, "metrics 客户端不应为 nil")

	// 5. 测试获取集群版本信息
	version, err := kubeClient.Discovery().ServerVersion()
	assert.NoError(t, err, "应能获取集群版本信息")
	t.Logf("集群版本: %s", version.String())

	// 6. 测试获取节点信息
	nodes, err := kubeClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	assert.NoError(t, err, "应能获取节点信息")
	t.Logf("集群节点数量: %d", len(nodes.Items))

	// 7. 测试获取默认命名空间的 Pod 列表
	pods, err := kubeClient.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	assert.NoError(t, err, "应能获取 Pod 列表")
	t.Logf("default 命名空间 Pod 数量: %d", len(pods.Items))

	// 8. 测试获取 nginx-deployment 的信息
	deployment, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-deployment", metav1.GetOptions{})
	assert.NoError(t, err, "应能获取 nginx-deployment")
	t.Logf("nginx-deployment 副本数: %d", deployment.Status.Replicas)

	// 9. 测试获取 Pod 指标
	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Logf("获取 Pod 指标失败: %v (这可能是因为 metrics-server 未安装)", err)
	} else {
		t.Logf("成功获取到 Pod 指标，数量: %d", len(podMetrics.Items))
		for _, metric := range podMetrics.Items {
			t.Logf("Pod: %s", metric.Name)
			for _, container := range metric.Containers {
				t.Logf("  Container: %s, CPU: %v, Memory: %v",
					container.Name,
					container.Usage.Cpu().String(),
					container.Usage.Memory().String())
			}
		}
	}
}
