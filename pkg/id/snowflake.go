// Package id 提供分布式唯一 ID 生成（雪花算法）。审计/证据链/成本行等主键复用。
// 用互斥锁串行化，优先保证正确性（P0 骨架量级；后续高吞吐可换无锁/分片）。
package id

import (
	"sync"
	"time"

	"github.com/openware-io/open-green-pass/pkg/clock"
)

const (
	nodeBits  uint64 = 10 // 1024 个节点
	seqBits   uint64 = 12 // 每毫秒 4096 序列
	nodeMax   int64  = -1 ^ (-1 << nodeBits)
	seqMask   int64  = -1 ^ (-1 << seqBits)
	timeShift        = nodeBits + seqBits
	nodeShift        = seqBits
	// epoch = 2024-01-01 00:00:00 UTC（毫秒）
	epoch int64 = 1704067200000
)

// Generator 是线程安全的雪花 ID 生成器。
type Generator struct {
	mu    sync.Mutex
	node  int64
	seq   int64
	last  int64 // 上一次时间戳(ms)
	clock clock.Clock
}

// New 创建绑定 node（0..1023）的生成器。
func New(node int64, c clock.Clock) (*Generator, error) {
	if node < 0 || node > nodeMax {
		return nil, &ErrNode{Node: node}
	}
	if c == nil {
		c = clock.System()
	}
	return &Generator{node: node, clock: c}, nil
}

// ErrNode 节点号越界。
type ErrNode struct{ Node int64 }

func (e *ErrNode) Error() string { return "id: invalid node" }

// Next 返回下一个唯一 ID。
func (g *Generator) Next() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.clock.Now().UnixMilli() - epoch
	// 时钟回拨：沿用 last，保证时间戳不倒退、ID 单调不重复。
	if now < g.last {
		now = g.last
	}
	if now == g.last {
		g.seq++
		if g.seq > seqMask {
			// 同一毫秒耗尽：等待进入下一毫秒
			for g.clock.Now().UnixMilli()-epoch <= g.last {
				time.Sleep(time.Millisecond)
			}
			now = g.clock.Now().UnixMilli() - epoch
			if now < g.last {
				now = g.last
			}
			g.seq = 0
			g.last = now
		}
	} else {
		g.seq = 0
		g.last = now
	}
	return g.compose(now, g.seq)
}

func (g *Generator) compose(ts, seq int64) int64 {
	return (ts << timeShift) | (g.node << nodeShift) | (seq & seqMask)
}
