package scaler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	autoscalingv1 "yemo.info/auto-scaling-system/api/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// PredictionResponse 定义预测服务的响应结构
type PredictionResponse struct {
	Values    []float64          `json:"values"`    // 预测值数组
	Features  map[string]float64 `json:"features"`  // 特征值
	Timestamp string             `json:"timestamp"` // 预测时间戳
}

// MetricsClient 定义指标客户端接口
type MetricsClient interface {
	GetPodMetrics(namespace string) (*metricsv1beta1.PodMetricsList, error)
}

// ScalingManager 管理伸缩决策
type ScalingManager struct {
	KubeClient      kubernetes.Interface
	MetricsClient   MetricsClient
	PredictorURL    string
	strategyFactory *StrategyFactory
}

// NewScalingManager 创建新的伸缩管理器
func NewScalingManager(kubeClient kubernetes.Interface, metricsClient MetricsClient, predictorURL string) *ScalingManager {
	return &ScalingManager{
		KubeClient:      kubeClient,
		MetricsClient:   metricsClient,
		PredictorURL:    predictorURL,
		strategyFactory: NewStrategyFactory(24*time.Hour, 5*time.Minute), // 24小时历史数据，5分钟采样间隔
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

// queryPrediction 从预测服务获取预测结果
func (s *ScalingManager) queryPrediction(metric string) (*PredictionResponse, error) {
	url := fmt.Sprintf("%s/predict?target=%s", s.PredictorURL, metric)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to query prediction service: %v", err)
	}
	defer resp.Body.Close()

	var result PredictionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode prediction response: %v", err)
	}
	return &result, nil
}

// CalculateDesiredReplicas 计算期望的副本数
func (s *ScalingManager) CalculateDesiredReplicas(hpa *autoscalingv1.HPAModifier, cpuUsage, memoryUsage float64) (int32, float64, error) {
	// 获取 CPU 和内存的预测结果
	cpuPrediction, err := s.queryPrediction("cpu")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get CPU prediction: %v", err)
	}

	memPrediction, err := s.queryPrediction("memory")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get memory prediction: %v", err)
	}

	// 计算最大预测负载
	var maxCPULoad, maxMemLoad float64
	for _, v := range cpuPrediction.Values {
		if v > maxCPULoad {
			maxCPULoad = v
		}
	}
	for _, v := range memPrediction.Values {
		if v > maxMemLoad {
			maxMemLoad = v
		}
	}

	// 计算 CPU 和内存的负载比率
	cpuRatio := maxCPULoad / hpa.Spec.CPUThreshold
	memRatio := maxMemLoad / hpa.Spec.MemoryThreshold

	// 使用较大的比率作为伸缩依据
	maxRatio := math.Max(cpuRatio, memRatio)

	// 计算期望的副本数
	currentReplicas := hpa.Status.CurrentReplicas
	desiredReplicas := int32(math.Ceil(float64(currentReplicas) * maxRatio))

	// 确保在最小和最大副本数范围内
	if desiredReplicas < hpa.Spec.MinReplicas {
		desiredReplicas = hpa.Spec.MinReplicas
	}
	if desiredReplicas > hpa.Spec.MaxReplicas {
		desiredReplicas = hpa.Spec.MaxReplicas
	}

	return desiredReplicas, maxRatio, nil
}

// ScaleWorkload 执行工作负载伸缩
func (s *ScalingManager) ScaleWorkload(ctx context.Context, hpa *autoscalingv1.HPAModifier) error {
	// 收集当前指标
	cpuUsage, memoryUsage, err := s.CollectMetrics(ctx, hpa)
	if err != nil {
		return fmt.Errorf("failed to collect metrics: %v", err)
	}

	// 获取工作负载的唯一标识
	workloadKey := fmt.Sprintf("%s/%s", hpa.Namespace, hpa.Spec.TargetRef.Name)

	// 获取当前工作负载的策略
	strategy := s.strategyFactory.GetStrategy(workloadKey, cpuUsage)

	// 计算期望副本数
	desiredReplicas, loadRatio, err := s.CalculateDesiredReplicas(hpa, cpuUsage, memoryUsage)
	if err != nil {
		return fmt.Errorf("failed to calculate desired replicas: %v", err)
	}

	// 检查是否需要预热
	if strategy.ShouldPreWarm() {
		// 获取预测结果
		cpuPrediction, err := s.queryPrediction("cpu")
		if err != nil {
			return fmt.Errorf("failed to get CPU prediction: %v", err)
		}

		// 如果预测到未来负载会超过阈值，提前扩容
		if len(cpuPrediction.Values) > 0 {
			maxPredictedLoad := 0.0
			for _, v := range cpuPrediction.Values {
				if v > maxPredictedLoad {
					maxPredictedLoad = v
				}
			}

			if maxPredictedLoad > strategy.GetScalingThreshold() {
				// 提前扩容到预测需要的副本数
				predictedReplicas := int32(math.Ceil(float64(hpa.Spec.MinReplicas) * maxPredictedLoad))
				if predictedReplicas > desiredReplicas {
					desiredReplicas = predictedReplicas
				}
			}
		}
	}

	// 获取当前副本数
	currentReplicas, err := s.getCurrentReplicas(ctx, hpa)
	if err != nil {
		return fmt.Errorf("failed to get current replicas: %v", err)
	}

	// 检查是否需要等待延迟时间
	if currentReplicas != desiredReplicas {
		// 获取上次伸缩时间
		lastScaledTime := hpa.Status.LastScaledTime
		if lastScaledTime != nil {
			// 检查是否已经过了延迟时间
			if time.Since(lastScaledTime.Time) < strategy.GetScalingDelay() {
				return nil // 等待延迟时间
			}
		}
	}

	// 更新工作负载的副本数
	if err := s.updateReplicas(ctx, hpa, desiredReplicas); err != nil {
		return fmt.Errorf("failed to update replicas: %v", err)
	}

	// 更新 HPA 状态
	hpa.Status.LastScaledTime = &metav1.Time{Time: time.Now()}
	hpa.Status.CurrentReplicas = desiredReplicas
	hpa.Status.PredictedLoad = loadRatio

	return nil
}

// getCurrentReplicas 获取当前副本数
func (s *ScalingManager) getCurrentReplicas(ctx context.Context, hpa *autoscalingv1.HPAModifier) (int32, error) {
	deployment, err := s.KubeClient.AppsV1().Deployments(hpa.Namespace).Get(ctx, hpa.Spec.TargetRef.Name, metav1.GetOptions{})
	if err != nil {
		return 0, err
	}
	return *deployment.Spec.Replicas, nil
}

// updateReplicas 更新工作负载的副本数
func (s *ScalingManager) updateReplicas(ctx context.Context, hpa *autoscalingv1.HPAModifier, desiredReplicas int32) error {
	scale, err := s.KubeClient.AppsV1().Deployments(hpa.Namespace).GetScale(ctx, hpa.Spec.TargetRef.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get scale subresource: %v", err)
	}

	scale.Spec.Replicas = desiredReplicas
	_, err = s.KubeClient.AppsV1().Deployments(hpa.Namespace).UpdateScale(ctx, hpa.Spec.TargetRef.Name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update scale subresource: %v", err)
	}

	return nil
}
