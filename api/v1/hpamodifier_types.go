package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HPAModifierSpec 定义 HPAModifier 的期望状态
type HPAModifierSpec struct {
	// TargetRef 指定要伸缩的工作负载（如 Deployment）
	TargetRef corev1.ObjectReference `json:"targetRef"`
	// MinReplicas 最小副本数
	MinReplicas int32 `json:"minReplicas"`
	// MaxReplicas 最大副本数
	MaxReplicas int32 `json:"maxReplicas"`
	// CPUThreshold CPU 使用率阈值，触发伸缩
	CPUThreshold float64 `json:"cpuThreshold"`
	// MemoryThreshold 内存使用率阈值，触发伸缩
	MemoryThreshold float64 `json:"memoryThreshold"`
	// PredictionWindow ARIMA 预测时间窗口（秒）
	PredictionWindow int32 `json:"predictionWindow"`
}

// HPAModifierStatus 定义 HPAModifier 的当前状态
type HPAModifierStatus struct {
	CurrentReplicas int32        `json:"currentReplicas"`
	PredictedLoad   float64      `json:"predictedLoad"`
	LastScaledTime  *metav1.Time `json:"lastScaledTime"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HPAModifier 是 hpamodifiers API 的模式
type HPAModifier struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HPAModifierSpec   `json:"spec,omitempty"`
	Status HPAModifierStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HPAModifierList 包含 HPAModifier 列表
type HPAModifierList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HPAModifier `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HPAModifier{}, &HPAModifierList{})
}
