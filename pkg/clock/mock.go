package clock

import "time"

// Mock 是可设置时间点的测试时钟。
type Mock struct{ t time.Time }

// NewMock 返回初始化为给定时刻的 Mock。
func NewMock(t time.Time) *Mock { return &Mock{t: t} }

// Now 返回当前设定时刻。
func (m *Mock) Now() time.Time { return m.t }

// Set 修改当前时刻。
func (m *Mock) Set(t time.Time) { m.t = t }
