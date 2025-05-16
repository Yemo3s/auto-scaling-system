package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

func main() {
	// 获取 kubeconfig 路径
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("获取用户目录失败: %v\n", err)
		os.Exit(1)
	}
	kubeconfig := filepath.Join(homeDir, ".kube", "config")

	// 创建 config
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Printf("创建配置失败: %v\n", err)
		os.Exit(1)
	}

	// 创建 Kubernetes 客户端
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("创建 Kubernetes 客户端失败: %v\n", err)
		os.Exit(1)
	}

	// 创建 metrics 客户端
	metricsClient, err := metrics.NewForConfig(config)
	if err != nil {
		fmt.Printf("创建 metrics 客户端失败: %v\n", err)
		os.Exit(1)
	}

	// 持续监控资源使用情况
	for {
		// 获取所有 Pod
		pods, err := kubeClient.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=nginx", // 只获取 nginx 相关的 Pod
		})
		if err != nil {
			fmt.Printf("获取 Pod 列表失败: %v\n", err)
			continue
		}

		// 获取 Pod 指标
		podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses("default").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Printf("获取 Pod 指标失败: %v\n", err)
			continue
		}

		// 清屏
		fmt.Print("\033[H\033[2J")

		// 打印时间戳
		fmt.Printf("时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
		fmt.Println("----------------------------------------")

		// 打印表头
		fmt.Printf("%-40s %-10s %-10s %-10s %-10s\n", "Pod名称", "状态", "CPU(核)", "内存(MB)", "重启次数")
		fmt.Println("----------------------------------------")

		// 遍历所有 Pod
		for _, pod := range pods.Items {
			// 查找对应的指标数据
			var cpuUsage, memoryUsage string
			for _, metric := range podMetrics.Items {
				if metric.Name == pod.Name {
					// 累加所有容器的资源使用
					var totalCPU int64
					var totalMemory int64
					for _, container := range metric.Containers {
						totalCPU += container.Usage.Cpu().MilliValue()
						totalMemory += container.Usage.Memory().Value()
					}
					cpuUsage = fmt.Sprintf("%.2f", float64(totalCPU)/1000)
					memoryUsage = fmt.Sprintf("%.1f", float64(totalMemory)/(1024*1024))
				}
			}

			// 如果没有找到指标数据，显示 N/A
			if cpuUsage == "" {
				cpuUsage = "N/A"
			}
			if memoryUsage == "" {
				memoryUsage = "N/A"
			}

			// 获取重启次数
			restarts := 0
			for _, containerStatus := range pod.Status.ContainerStatuses {
				restarts += int(containerStatus.RestartCount)
			}

			// 打印 Pod 信息
			fmt.Printf("%-40s %-10s %-10s %-10s %-10d\n",
				pod.Name,
				pod.Status.Phase,
				cpuUsage,
				memoryUsage,
				restarts,
			)
		}

		// 等待 2 秒后继续
		time.Sleep(2 * time.Second)
	}
}
