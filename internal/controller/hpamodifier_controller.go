package controller

import (
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	"time"
	metrics2 "yemo.info/auto-scaling-system/internal/metrics"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	autoscalingv1 "yemo.info/auto-scaling-system/api/v1"
	"yemo.info/auto-scaling-system/internal/scaler"
)

// 定义伸缩稳定性的常量
const (
	RequeueInterval = 10 * time.Second                                          // 默认重新调度间隔：10秒
	PredictorURL    = "http://predictor-service.default.svc.cluster.local:8000" // 预测服务的URL
)

// HPAModifierReconciler 用于调谐 HPAModifier 对象
type HPAModifierReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	ScalingMgr    *scaler.ScalingManager
	KubeClient    kubernetes.Interface
	MetricsClient metrics.Interface
}

//+kubebuilder:rbac:groups=autoscaling.yemo.info,resources=hpamodifiers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=autoscaling.yemo.info,resources=hpamodifiers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=metrics.k8s.io,resources=pods,verbs=get;list

// Reconcile 是控制器调谐的主逻辑
func (r *HPAModifierReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("hpamodifier", req.NamespacedName)

	// 获取 HPAModifier 实例
	hpaModifier := &autoscalingv1.HPAModifier{}
	if err := r.Get(ctx, req.NamespacedName, hpaModifier); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "无法获取 HPAModifier")
		return ctrl.Result{}, err
	}

	// 使用伸缩管理器执行伸缩
	if err := r.ScalingMgr.ScaleWorkload(ctx, hpaModifier); err != nil {
		log.Error(err, "伸缩失败")
		return ctrl.Result{}, err
	}

	// 更新状态
	if err := r.Status().Update(ctx, hpaModifier); err != nil {
		log.Error(err, "更新状态失败")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: RequeueInterval}, nil
}

// SetupWithManager 设置控制器与管理器
func (r *HPAModifierReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// 创建 MetricsClient 适配器
	metricsClient := metrics2.NewK8sMetricsClient(r.MetricsClient)

	// 初始化伸缩管理器
	r.ScalingMgr = scaler.NewScalingManager(r.KubeClient, metricsClient, PredictorURL)

	return ctrl.NewControllerManagedBy(mgr).
		For(&autoscalingv1.HPAModifier{}).
		Complete(r)
}
