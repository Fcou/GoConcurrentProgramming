package Channel

import "unsafe"

//初始化
func makechan(t *chantype, size int) *hchan {
	elem := t.elem

	// 略去检查代码
	mem, overflow := math.MulUintptr(elem.size, uintptr(size))

	//
	var c *hchan
	switch {
	case mem == 0:
		// chan的size或者元素的size是0，不必创建buf
		c = (*hchan)(mallocgc(hchanSize, nil, true))
		c.buf = c.raceaddr()
	case elem.ptrdata == 0:
		// 元素不是指针，分配一块连续的内存给hchan数据结构和buf
		c = (*hchan)(mallocgc(hchanSize+mem, nil, true))
		// hchan数据结构后面紧接着就是buf
		c.buf = add(unsafe.Pointer(c), hchanSize)
	default:
		// 元素包含指针，那么单独分配buf
		c = new(hchan)
		c.buf = mallocgc(mem, elem, true)
	}

	// 元素大小、类型、容量都记录下来
	c.elemsize = uint16(elem.size)
	c.elemtype = elem
	c.dataqsiz = uint(size)
	lockInit(&c.lock, lockRankHchan)

	return c
}

//Go 在编译发送数据给 chan 的时候，会把 send 语句转换成 chansend1 函数，chansend1 函数会调用 chansend
func chansend1(c *hchan, elem unsafe.Pointer) {
	chansend(c, elem, true, getcallerpc())
}
func chansend(c *hchan, ep unsafe.Pointer, block bool, callerpc uintptr) bool {
	// 第一部分
	if c == nil {
		if !block {
			return false
		}
		gopark(nil, nil, waitReasonChanSendNilChan, traceEvGoStop, 2)
		throw("unreachable")
	}

	// 第二部分，如果chan没有被close,并且chan满了，直接返回
	if !block && c.closed == 0 && full(c) {
		return false
	}

	// 第三部分，chan已经被close的情景
	lock(&c.lock) // 开始加锁
	if c.closed != 0 {
		unlock(&c.lock)
		panic(plainError("send on closed channel"))
	}

	// 第四部分，从接收队列中出队一个等待的receiver
	if sg := c.recvq.dequeue(); sg != nil {
		//
		send(c, sg, ep, func() { unlock(&c.lock) }, 3)
		return true
	}

	// 第五部分，buf还没满
	if c.qcount < c.dataqsiz {
		qp := chanbuf(c, c.sendx)
		if raceenabled {
			raceacquire(qp)
			racerelease(qp)
		}
		typedmemmove(c.elemtype, qp, ep)
		c.sendx++
		if c.sendx == c.dataqsiz {
			c.sendx = 0
		}
		c.qcount++
		unlock(&c.lock)
		return true
	}

	// 第六部分，buf满。
	// chansend1不会进入if块里，因为chansend1的block=true
	if !block {
		unlock(&c.lock)
		return false
	}

}

//在处理从 chan 中接收数据时，Go 会把代码转换成 chanrecv1 函数，如果要返回两个返回值，会转换成 chanrecv2，chanrecv1 函数和 chanrecv2 会调用 chanrecv。
func chanrecv1(c *hchan, elem unsafe.Pointer) {
	chanrecv(c, elem, true)
}
func chanrecv2(c *hchan, elem unsafe.Pointer) (received bool) {
	_, received = chanrecv(c, elem, true)
	return
}

func chanrecv(c *hchan, ep unsafe.Pointer, block bool) (selected, received bool) {
	// 第一部分，chan为nil
	if c == nil {
		if !block {
			return
		}
		gopark(nil, nil, waitReasonChanReceiveNilChan, traceEvGoStop, 2)
		throw("unreachable")
	}

	// 第二部分, block=false且c为空
	if !block && empty(c) {
		//....
	}

	// 加锁，返回时释放锁
	lock(&c.lock)
	// 第三部分，c已经被close,且chan为空empty
	if c.closed != 0 && c.qcount == 0 {
		unlock(&c.lock)
		if ep != nil {
			typedmemclr(c.elemtype, ep)
		}
		return true, false
	}

	// 第四部分，如果sendq队列中有等待发送的sender
	if sg := c.sendq.dequeue(); sg != nil {
		recv(c, sg, ep, func() { unlock(&c.lock) }, 3)
		return true, true
	}

	// 第五部分, 没有等待的sender, buf中有数据
	if c.qcount > 0 {
		qp := chanbuf(c, c.recvx)
		if ep != nil {
			typedmemmove(c.elemtype, ep, qp)
		}
		typedmemclr(c.elemtype, qp)
		c.recvx++
		if c.recvx == c.dataqsiz {
			c.recvx = 0
		}
		c.qcount--
		unlock(&c.lock)
		return true, true
	}

	if !block {
		unlock(&c.lock)
		return false, false
	}

	// 第六部分， buf中没有元素，阻塞
	// ......

}


//通过 close 函数，可以把 chan 关闭，编译器会替换成 closechan 方法的调用。
func closechan(c *hchan) {
    if c == nil { // chan为nil, panic
      panic(plainError("close of nil channel"))
    }
  
    lock(&c.lock)
    if c.closed != 0 {// chan已经closed, panic
      unlock(&c.lock)
      panic(plainError("close of closed channel"))
    }

    c.closed = 1  

    var glist gList

    // 释放所有的reader
    for {
      sg := c.recvq.dequeue()
      ......
      gp := sg.g
      ......
      glist.push(gp)
    }
  
    // 释放所有的writer (它们会panic)
    for {
      sg := c.sendq.dequeue()
      ......
      gp := sg.g
      ......
      glist.push(gp)
    }
    unlock(&c.lock)
  
    for !glist.empty() {
      gp := glist.pop()
      gp.schedlink = 0
      goready(gp, 3)
    }
  }