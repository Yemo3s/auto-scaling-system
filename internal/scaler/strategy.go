package scaler

import (
	"time"
)

// ScalingStrategy 定义伸缩策略接口
type ScalingStrategy interface {
	// GetScalingDelay 获取伸缩延迟时间
	GetScalingDelay() time.Duration
	// GetScalingThreshold 获取伸缩阈值
	GetScalingThreshold() float64
	// ShouldPreWarm 是否应该预热
	ShouldPreWarm() bool
	// GetPreWarmTime 获取预热时间
	GetPreWarmTime() time.Duration
}

// StableStrategy 稳定型策略
type StableStrategy struct {
	baseDelay     time.Duration
	baseThreshold float64
}

func NewStableStrategy() *StableStrategy {
	return &StableStrategy{
		baseDelay:     5 * time.Minute, // 较长的延迟
		baseThreshold: 0.8,             // 较高的阈值
	}
}

func (s *StableStrategy) GetScalingDelay() time.Duration {
	return s.baseDelay
}

func (s *StableStrategy) GetScalingThreshold() float64 {
	return s.baseThreshold
}

func (s *StableStrategy) ShouldPreWarm() bool {
	return false
}

func (s *StableStrategy) GetPreWarmTime() time.Duration {
	return 0
}

// PeriodicStrategy 周期型策略
type PeriodicStrategy struct {
	baseDelay     time.Duration
	baseThreshold float64
}

func NewPeriodicStrategy() *PeriodicStrategy {
	return &PeriodicStrategy{
		baseDelay:     2 * time.Minute, // 中等延迟
		baseThreshold: 0.7,             // 中等阈值
	}
}

func (s *PeriodicStrategy) GetScalingDelay() time.Duration {
	return s.baseDelay
}

func (s *PeriodicStrategy) GetScalingThreshold() float64 {
	return s.baseThreshold
}

func (s *PeriodicStrategy) ShouldPreWarm() bool {
	return true
}

func (s *PeriodicStrategy) GetPreWarmTime() time.Duration {
	return 15 * time.Minute // 提前15分钟预热
}

// BurstStrategy 突发型策略
type BurstStrategy struct {
	baseDelay     time.Duration
	baseThreshold float64
}

func NewBurstStrategy() *BurstStrategy {
	return &BurstStrategy{
		baseDelay:     30 * time.Second, // 较短的延迟
		baseThreshold: 0.6,              // 较低的阈值
	}
}

func (s *BurstStrategy) GetScalingDelay() time.Duration {
	return s.baseDelay
}

func (s *BurstStrategy) GetScalingThreshold() float64 {
	return s.baseThreshold
}

func (s *BurstStrategy) ShouldPreWarm() bool {
	return false
}

func (s *BurstStrategy) GetPreWarmTime() time.Duration {
	return 0
}

// StrategyFactory 策略工厂
type StrategyFactory struct {
	patternAnalyzer *PatternAnalyzer
}

func NewStrategyFactory(historyWindow, sampleInterval time.Duration) *StrategyFactory {
	return &StrategyFactory{
		patternAnalyzer: NewPatternAnalyzer(historyWindow, sampleInterval),
	}
}

// GetStrategy 根据工作负载模式获取对应的策略
func (f *StrategyFactory) GetStrategy(workloadKey string, currentValue float64) ScalingStrategy {
	pattern := f.patternAnalyzer.AnalyzePattern(workloadKey, currentValue)

	switch pattern {
	case PatternStable:
		return NewStableStrategy()
	case PatternPeriodic:
		return NewPeriodicStrategy()
	case PatternBurst:
		return NewBurstStrategy()
	default:
		return NewStableStrategy() // 默认使用稳定型策略
	}
}
