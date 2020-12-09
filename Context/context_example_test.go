package Context

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestContextWithValue(t *testing.T) {

	ctx := context.TODO()
	ctx = context.WithValue(ctx, "key1", "0001")
	ctx = context.WithValue(ctx, "key2", "0001")
	ctx = context.WithValue(ctx, "key3", "0001")
	ctx = context.WithValue(ctx, "key1", "0004")

	fmt.Println(ctx.Value("key1"))
}

func TestContextWithCancel(t *testing.T) {
	gen := func(ctx context.Context) <-chan int {
		dst := make(chan int)
		n := 1
		go func() {
			for {
				select {
				case <-ctx.Done():
					return // returning not to leak the goroutine
				case dst <- n:
					n++
				}
			}
		}()
		return dst
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	for n := range gen(ctx) {
		fmt.Println(n)
		if n == 5 {
			break
		}
	}
}

// func slowOperationWithTimeout(ctx context.Context) (Result, error) {
// 	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
// 	defer cancel() // 一旦慢操作完成就立马调用cancel
// 	return slowOperation(ctx)
// }

func TestContexExample(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer func() {
			fmt.Println("goroutine exit")
		}()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(time.Second)
			}
		}
	}()

	time.Sleep(time.Second)
	cancel()
	time.Sleep(2 * time.Second)
}

func TestContexCancle(t *testing.T) {

	pctx, cancel := context.WithCancel(context.Background())
	cctx := context.WithValue(pctx, "key1", "0001")
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		<-cctx.Done()
		time.Sleep(time.Second * 1)
		fmt.Println("cctx done")

		if cctx.Value("key1") == "0001" {
			fmt.Println("OKK")
		}
		time.Sleep(time.Second * 5)
		wg.Done()
	}()

	time.Sleep(time.Second * 1)
	cancel()
	fmt.Println("pctx cancel")

	wg.Wait()
}
