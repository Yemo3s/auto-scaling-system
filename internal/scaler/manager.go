package scaler

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	autoscalingv1 "yemo.info/auto-scaling-system/api/v1"
	"yemo.info/auto-scaling-system/internal/predictor"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// MetricsClient 定义指标客户端接口
type MetricsClient interface {
	GetPodMetrics(namespace string) (*metricsv1beta1.PodMetricsList, error)
}

// ScalingManager 管理伸缩决策
type ScalingManager struct {
	KubeClient    kubernetes.Interface
	MetricsClient MetricsClient
	predictor     *predictor.ARIMAPredictor
	dataWindow    time.Duration
}

// NewScalingManager 创建新的伸缩管理器
func NewScalingManager(kubeClient kubernetes.Interface, metricsClient MetricsClient) *ScalingManager {
	return &ScalingManager{
		KubeClient:    kubeClient,
		MetricsClient: metricsClient,
		predictor:     predictor.NewARIMAPredictor(2, 1, 1, false), // 使用ARIMA(2,1,1)模型
		dataWindow:    10 * time.Minute,                            // 使用10分钟的数据窗口
	}
}

// CollectMetrics 收集目标工作负载的指标
func (s *ScalingManager) CollectMetrics(ctx context.Context, hpa *autoscalingv1.HPAModifier) (float64, float64, error) {
	podMetrics, err := s.MetricsClient.GetPodMetrics(hpa.Spec.TargetRef.Namespace)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get pod metrics: %v", err)
	}

	var totalCPU, totalMemory resource.Quantity
	podCount := 0
	for _, pod := range podMetrics.Items {
		// 使用更可靠的标签匹配逻辑
		if pod.Labels["app"] == hpa.Spec.TargetRef.Name ||
			strings.HasPrefix(pod.Name, hpa.Spec.TargetRef.Name+"-") {
			for _, container := range pod.Containers {
				cpu := container.Usage.Cpu()
				memory := container.Usage.Memory()
				totalCPU.Add(*cpu)
				totalMemory.Add(*memory)
			}
			podCount++
		}
	}

	if podCount == 0 {
		return 0, 0, fmt.Errorf("no pods found for deployment %s", hpa.Spec.TargetRef.Name)
	}

	cpuUsage := float64(totalCPU.MilliValue()) / float64(podCount) / 1000.0
	memoryUsage := float64(totalMemory.Value()) / float64(podCount) / (1024 * 1024 * 1024) // 转换为GB

	return cpuUsage, memoryUsage, nil
}

// CalculateDesiredReplicas 计算期望的副本数
func (s *ScalingManager) CalculateDesiredReplicas(hpa *autoscalingv1.HPAModifier, cpuUsage, memoryUsage float64) (int32, float64, error) {
	// 添加当前使用率到预测器
	s.predictor.AddDataPoint(time.Now(), math.Max(cpuUsage/hpa.Spec.CPUThreshold, memoryUsage/hpa.Spec.MemoryThreshold))

	// 预测未来负载
	predictions, err := s.predictor.Predict(int(hpa.Spec.PredictionWindow))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to predict load: %v", err)
	}

	// 找出预测期间的最大负载
	var maxPredictedLoad float64
	for _, p := range predictions {
		if p.Value > maxPredictedLoad {
			maxPredictedLoad = p.Value
		}
	}

	// 基于当前负载和预测负载计算所需副本数
	currentReplicas := hpa.Status.CurrentReplicas
	desiredReplicas := int32(math.Ceil(float64(currentReplicas) * maxPredictedLoad))

	// 确保在最小和最大副本数范围内
	if desiredReplicas < hpa.Spec.MinReplicas {
		desiredReplicas = hpa.Spec.MinReplicas
	}
	if desiredReplicas > hpa.Spec.MaxReplicas {
		desiredReplicas = hpa.Spec.MaxReplicas
	}

	return desiredReplicas, maxPredictedLoad, nil
}

// ScaleWorkload 执行工作负载伸缩
func (s *ScalingManager) ScaleWorkload(ctx context.Context, hpa *autoscalingv1.HPAModifier) error {
	// 收集当前指标
	cpuUsage, memoryUsage, err := s.CollectMetrics(ctx, hpa)
	if err != nil {
		return err
	}

	// 计算期望副本数
	desiredReplicas, maxPredictedLoad, err := s.CalculateDesiredReplicas(hpa, cpuUsage, memoryUsage)
	if err != nil {
		return err
	}

	// 如果副本数没有变化，直接返回
	if desiredReplicas == hpa.Status.CurrentReplicas {
		return nil
	}

	// 更新工作负载的副本数
	scale, err := s.KubeClient.AppsV1().Deployments(hpa.Spec.TargetRef.Namespace).GetScale(ctx, hpa.Spec.TargetRef.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get scale subresource: %v", err)
	}

	scale.Spec.Replicas = desiredReplicas
	_, err = s.KubeClient.AppsV1().Deployments(hpa.Spec.TargetRef.Namespace).UpdateScale(ctx, hpa.Spec.TargetRef.Name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update scale subresource: %v", err)
	}

	// 更新HPA状态
	hpa.Status.CurrentReplicas = desiredReplicas
	hpa.Status.PredictedLoad = maxPredictedLoad
	hpa.Status.LastScaledTime = &metav1.Time{Time: time.Now()}

	return nil
}
