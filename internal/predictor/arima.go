package predictor

import (
	"fmt"
	"math"
	"time"

	"gonum.org/v1/gonum/stat"
)

// TimeSeriesData 表示时间序列数据点
type TimeSeriesData struct {
	Timestamp time.Time
	Value     float64
}

// ARIMAPredictor ARIMA模型预测器
type ARIMAPredictor struct {
	p, d, q    int // ARIMA模型参数
	data       []float64
	timestamps []time.Time
	seasonal   bool
}

// NewARIMAPredictor 创建新的ARIMA预测器
func NewARIMAPredictor(p, d, q int, seasonal bool) *ARIMAPredictor {
	return &ARIMAPredictor{
		p:          p,
		d:          d,
		q:          q,
		seasonal:   seasonal,
		data:       make([]float64, 0),
		timestamps: make([]time.Time, 0),
	}
}

// AddDataPoint 添加新的数据点
func (a *ARIMAPredictor) AddDataPoint(timestamp time.Time, value float64) {
	a.data = append(a.data, value)
	a.timestamps = append(a.timestamps, timestamp)
}

// difference 计算时间序列的差分
func (a *ARIMAPredictor) difference(data []float64, order int) []float64 {
	if order == 0 {
		return data
	}

	diff := make([]float64, len(data)-1)
	for i := 0; i < len(data)-1; i++ {
		diff[i] = data[i+1] - data[i]
	}

	return a.difference(diff, order-1)
}

// autoCorrelation 计算自相关系数
func (a *ARIMAPredictor) autoCorrelation(data []float64, lag int) float64 {
	n := len(data)
	if lag >= n {
		return 0
	}

	mean := stat.Mean(data, nil)
	var numerator, denominator float64

	for i := 0; i < n-lag; i++ {
		numerator += (data[i] - mean) * (data[i+lag] - mean)
	}

	for i := 0; i < n; i++ {
		denominator += math.Pow(data[i]-mean, 2)
	}

	if denominator == 0 {
		return 0
	}

	return numerator / denominator
}

// Predict 预测未来值
func (a *ARIMAPredictor) Predict(steps int) ([]TimeSeriesData, error) {
	if len(a.data) < a.p+a.d+a.q {
		return nil, fmt.Errorf("insufficient data points for prediction")
	}

	// 进行差分
	diffData := a.difference(a.data, a.d)

	// 计算AR系数
	arCoef := make([]float64, a.p)
	for i := 0; i < a.p; i++ {
		arCoef[i] = a.autoCorrelation(diffData, i+1)
	}

	// 预测未来值
	predictions := make([]TimeSeriesData, steps)
	lastTimestamp := a.timestamps[len(a.timestamps)-1]
	interval := lastTimestamp.Sub(a.timestamps[len(a.timestamps)-2])

	for i := 0; i < steps; i++ {
		var prediction float64
		// 使用AR模型进行预测
		for j := 0; j < a.p; j++ {
			if len(diffData)-j-1 >= 0 {
				prediction += arCoef[j] * diffData[len(diffData)-j-1]
			}
		}

		// 还原差分
		for d := 0; d < a.d; d++ {
			prediction += a.data[len(a.data)-1]
		}

		predictions[i] = TimeSeriesData{
			Timestamp: lastTimestamp.Add(interval * time.Duration(i+1)),
			Value:     prediction,
		}
	}

	return predictions, nil
}

// CalculateError 计算预测误差
func (a *ARIMAPredictor) CalculateError(actual, predicted float64) float64 {
	return math.Abs(actual-predicted) / actual
}
