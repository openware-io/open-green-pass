// Package hashchain 提供审计证据链的哈希链（SHA-256）。
// 每个写入块关联前一块的哈希，形成不可篡改的链条；VerifyChain 逐块重算校验。
package hashchain

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// Block 是链上的一个元素。
type Block struct {
	Seq      int64
	PrevHash string
	Data     []byte
	Hash     string
}

// Append 计算新块哈希 = sha256(seq || prevHash || data)。
func Append(seq int64, prevHash string, data []byte) Block {
	h := sha256.New()
	h.Write([]byte(strconv.FormatInt(seq, 10)))
	h.Write([]byte(prevHash))
	h.Write(data)
	return Block{Seq: seq, PrevHash: prevHash, Data: data, Hash: hex.EncodeToString(h.Sum(nil))}
}

// Verify 校验单个块：重算哈希并与记录比对。
func Verify(b Block) bool {
	re := Append(b.Seq, b.PrevHash, b.Data)
	return re.Hash == b.Hash
}

// VerifyChain 从 genesis 逐块校验整链连续性与完整性。
// 首个块 PrevHash 应等于 genesisHash（外部校验锚点），此处校验 i>0 块与前一哈希衔接。
func VerifyChain(blocks []Block) bool {
	for i, b := range blocks {
		if !Verify(b) {
			return false
		}
		if i > 0 && b.PrevHash != blocks[i-1].Hash {
			return false
		}
	}
	return true
}
