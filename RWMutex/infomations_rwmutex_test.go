// 获取等待者的数量等指标
package RWMutex

import (
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"
)

type RWMutex struct {
	w           Mutex  // 互斥锁解决多个writer的竞争
	writerSem   uint32 // writer信号量
	readerSem   uint32 // reader信号量
	readerCount int32  // reader的数量
	readerWait  int32  // writer等待完成的reader的数量
}

func (m *RWMutex) GetReaderCount() int32 {
	// readerCount 这个成员变量前有1个mutex+2个uint32
	readerCount := atomic.LoadInt32((*int32)(unsafe.Pointer(uintptr(unsafe.Pointer(&m)) + unsafe.Sizeof(sync.Mutex{}) + 2*unsafe.Sizeof(uint32(0)))))
	return readerCount
}

func (m *Mutex) GetReaderWait() int32 {
	// readerWait 这个成员变量前有1个mutex+2个uint32+1个int32
	readerWait := atomic.LoadInt32((*int32)(unsafe.Pointer(uintptr(unsafe.Pointer(&m)) + unsafe.Sizeof(sync.Mutex{}) + 2*unsafe.Sizeof(uint32(0)) + unsafe.Sizeof(int32(0)))))
	return readerWait
}

func TestGetInfomations(t *testing.T) {

}
