package Semaphore

import (
	"container/list"
	"context"
	"sync"
)

type waiter struct {
	n     int64
	ready chan<- struct{} // Closed when semaphore acquired.
}

// NewWeighted creates a new weighted semaphore with the given
// maximum combined weight for concurrent access.
func NewWeighted(n int64) *Weighted {
	w := &Weighted{size: n}
	return w
}

type Weighted struct {
	size    int64      // 最大资源数
	cur     int64      // 当前已被使用的资源
	mu      sync.Mutex // 互斥锁，对字段的保护
	waiters list.List  // 等待队列
}

func (s *Weighted) Acquire(ctx context.Context, n int64) error {
	s.mu.Lock()
	// fast path, 如果有足够的资源，都不考虑ctx.Done的状态，将cur加上n就返回
	if s.size-s.cur >= n && s.waiters.Len() == 0 {
		s.cur += n
		s.mu.Unlock()
		return nil
	}

	// 如果是不可能完成的任务，请求的资源数大于能提供的最大的资源数
	if n > s.size {
		s.mu.Unlock()
		// 依赖ctx的状态返回，否则一直等待
		<-ctx.Done()
		return ctx.Err()
	}

	// 否则就需要把调用者加入到等待队列中
	// 创建了一个ready chan,以便被通知唤醒
	ready := make(chan struct{})
	w := waiter{n: n, ready: ready}
	elem := s.waiters.PushBack(w)
	s.mu.Unlock()

	// 等待
	select {
	case <-ctx.Done(): // context的Done被关闭
		err := ctx.Err()
		s.mu.Lock()
		select {
		case <-ready: // 如果被唤醒了，忽略ctx的状态
			err = nil
		default:
			通知waiter
			isFront := s.waiters.Front() == elem
			s.waiters.Remove(elem)
			// 通知其它的waiters,检查是否有足够的资源
			if isFront && s.size > s.cur {
				s.notifyWaiters()
			}
		}
		s.mu.Unlock()
		return err
	case <-ready: // 被唤醒了
		return nil
	}
}

func (s *Weighted) Release(n int64) {
	s.mu.Lock()
	s.cur -= n
	if s.cur < 0 {
		s.mu.Unlock()
		panic("semaphore: released more than held")
	}
	s.notifyWaiters()
	s.mu.Unlock()
}

func (s *Weighted) notifyWaiters() {
	for {
		next := s.waiters.Front() //取出等待队列的第一元素
		if next == nil {
			break // No more waiters blocked.
		}

		w := next.Value.(waiter)
		if s.size-s.cur < w.n {
			//避免饥饿，这里还是按照先入先出的方式处理
			break
		}

		s.cur += w.n
		s.waiters.Remove(next)
		close(w.ready) //通过close chan ,从而 <- chan 不再阻塞，Acquire不再阻塞
	}
}

// TryAcquire acquires the semaphore with a weight of n without blocking.
// On success, returns true. On failure, returns false and leaves the semaphore unchanged.
func (s *Weighted) TryAcquire(n int64) bool {
	s.mu.Lock()
	success := s.size-s.cur >= n && s.waiters.Len() == 0
	if success {
		s.cur += n
	}
	s.mu.Unlock()
	return success
}
