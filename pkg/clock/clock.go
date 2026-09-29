// Package clock 提供可注入的时钟抽象，便于测试固定时间。
package clock

import "time"

// Clock 抽象时间来源。
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

// Now 返回当前时间。
func (systemClock) Now() time.Time { return time.Now() }

// System 返回系统时钟（生产默认）。
func System() Clock { return systemClock{} }
