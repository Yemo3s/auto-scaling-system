package scaler

import (
	"math"
	"time"
)

// WorkloadPattern 定义工作负载的使用模式
type WorkloadPattern int

const (
	// PatternStable 稳定型：资源使用相对稳定
	PatternStable WorkloadPattern = iota
	// PatternPeriodic 周期型：有明显的周期性波动
	PatternPeriodic
	// PatternBurst 突发型：偶发性瞬时高负载
	PatternBurst
)

// PatternAnalyzer 分析工作负载模式
type PatternAnalyzer struct {
	// 历史数据窗口大小
	historyWindow time.Duration
	// 采样间隔
	sampleInterval time.Duration
	// 历史数据
	historyData map[string][]float64
}

// NewPatternAnalyzer 创建新的模式分析器
func NewPatternAnalyzer(historyWindow, sampleInterval time.Duration) *PatternAnalyzer {
	return &PatternAnalyzer{
		historyWindow:  historyWindow,
		sampleInterval: sampleInterval,
		historyData:    make(map[string][]float64),
	}
}

// AnalyzePattern 分析工作负载模式
func (pa *PatternAnalyzer) AnalyzePattern(workloadKey string, currentValue float64) WorkloadPattern {
	// 更新历史数据
	if _, exists := pa.historyData[workloadKey]; !exists {
		pa.historyData[workloadKey] = make([]float64, 0)
	}
	pa.historyData[workloadKey] = append(pa.historyData[workloadKey], currentValue)

	// 保持历史数据在窗口范围内
	windowSize := int(pa.historyWindow / pa.sampleInterval)
	if len(pa.historyData[workloadKey]) > windowSize {
		pa.historyData[workloadKey] = pa.historyData[workloadKey][len(pa.historyData[workloadKey])-windowSize:]
	}

	// 分析模式
	return pa.determinePattern(workloadKey)
}

// determinePattern 确定工作负载模式
func (pa *PatternAnalyzer) determinePattern(workloadKey string) WorkloadPattern {
	data := pa.historyData[workloadKey]
	if len(data) < 2 {
		return PatternStable // 数据不足时默认为稳定型
	}

	// 计算统计指标
	mean := calculateMean(data)
	stdDev := calculateStdDev(data, mean)
	cv := stdDev / mean // 变异系数

	// 检测周期性
	isPeriodic := detectPeriodicity(data)

	// 检测突发性
	isBurst := detectBurst(data, mean, stdDev)

	// 根据特征判断模式
	if isBurst {
		return PatternBurst
	} else if isPeriodic {
		return PatternPeriodic
	} else if cv < 0.2 { // 变异系数小于0.2认为是稳定的
		return PatternStable
	} else {
		return PatternPeriodic // 默认归类为周期型
	}
}

// calculateMean 计算平均值
func calculateMean(data []float64) float64 {
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

// calculateStdDev 计算标准差
func calculateStdDev(data []float64, mean float64) float64 {
	sum := 0.0
	for _, v := range data {
		diff := v - mean
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(data)))
}

// detectPeriodicity 检测周期性
func detectPeriodicity(data []float64) bool {
	if len(data) < 4 {
		return false
	}

	// 使用自相关函数检测周期性
	autocorr := make([]float64, len(data)/2)
	mean := calculateMean(data)

	for lag := 1; lag < len(autocorr); lag++ {
		numerator := 0.0
		denominator := 0.0

		for i := 0; i < len(data)-lag; i++ {
			numerator += (data[i] - mean) * (data[i+lag] - mean)
			denominator += (data[i] - mean) * (data[i] - mean)
		}

		if denominator == 0 {
			continue
		}
		autocorr[lag] = numerator / denominator
	}

	// 检查自相关函数是否有明显的周期性峰值
	peakCount := 0
	for i := 1; i < len(autocorr)-1; i++ {
		if autocorr[i] > autocorr[i-1] && autocorr[i] > autocorr[i+1] && autocorr[i] > 0.5 {
			peakCount++
		}
	}

	return peakCount >= 2
}

// detectBurst 检测突发性
func detectBurst(data []float64, mean, stdDev float64) bool {
	if len(data) < 2 {
		return false
	}

	// 计算相邻点之间的变化率
	changes := make([]float64, len(data)-1)
	for i := 0; i < len(data)-1; i++ {
		changes[i] = math.Abs(data[i+1] - data[i])
	}

	// 计算变化率的统计特征
	changeMean := calculateMean(changes)
	changeStdDev := calculateStdDev(changes, changeMean)

	// 如果变化率的标准差很大，说明存在突发性
	return changeStdDev > 2*changeMean
}
