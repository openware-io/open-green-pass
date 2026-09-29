package hashchain

import (
	"bytes"
	"testing"
)

func TestAppendVerify(t *testing.T) {
	genesis := Append(0, "", []byte("boot"))
	if !Verify(genesis) {
		t.Fatal("genesis 校验失败")
	}
	next := Append(1, genesis.Hash, []byte("run 1"))
	if !Verify(next) {
		t.Fatal("next 校验失败")
	}
	if next.PrevHash != genesis.Hash {
		t.Fatalf("next.PrevHash=%q != genesis.Hash=%q", next.PrevHash, genesis.Hash)
	}
	if !VerifyChain([]Block{genesis, next}) {
		t.Fatal("整链校验失败")
	}
}

func TestTamperDetected(t *testing.T) {
	genesis := Append(0, "", []byte("boot"))
	next := Append(1, genesis.Hash, []byte("run 1"))
	// 篡改 data
	tampered := next
	tampered.Data = []byte("run 1 (fake)")
	if Verify(tampered) {
		t.Fatal("篡改 data 后应校验失败")
	}
	// 篡改 seq
	tampered2 := next
	tampered2.Seq = 99
	if Verify(tampered2) {
		t.Fatal("篡改 seq 后应校验失败")
	}
}

func TestChainBreak(t *testing.T) {
	a := Append(0, "", []byte("x"))
	b := Append(1, "wrong-prev", []byte("y"))
	if VerifyChain([]Block{a, b}) {
		t.Fatal("PrevHash 断裂应校验失败")
	}
	if !VerifyChain([]Block{a}) {
		t.Fatal("单块应可校验")
	}
}

func TestDataPreserved(t *testing.T) {
	g := Append(0, "", []byte("payload-1"))
	if !bytes.Equal(g.Data, []byte("payload-1")) {
		t.Fatal("Data 未保留")
	}
}
