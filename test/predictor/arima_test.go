package predictor_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"yemo.info/auto-scaling-system/internal/predictor"
)

func TestARIMAPredictor(t *testing.T) {
	// 创建预测器
	p := predictor.NewARIMAPredictor(2, 1, 1, false)

	// 添加测试数据
	now := time.Now()
	testData := []float64{1.0, 1.2, 1.5, 1.8, 2.0, 2.3, 2.5}
	for i, value := range testData {
		p.AddDataPoint(now.Add(time.Duration(i)*time.Minute), value)
	}

	// 测试预测
	predictions, err := p.Predict(3)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(predictions))

	// 验证预测值是否合理
	for _, pred := range predictions {
		assert.True(t, pred.Value > 0)
		assert.True(t, pred.Timestamp.After(now))
	}

	// 测试预测误差
	error := p.CalculateError(2.7, predictions[0].Value)
	assert.True(t, error >= 0 && error <= 1.0)
}
