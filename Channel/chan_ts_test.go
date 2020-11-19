package Channel

import (
	"fmt"
	"testing"
	"time"
)

//有四个 goroutine，编号为 1、2、3、4。每秒钟会有一个 goroutine 打印出它自己的编号，要求你编写一个程序，让输出的编号总是按照 1、2、3、4、1、2、3、4、……的顺序打印出来。
func TestTaskScheduling(t *testing.T) {
	ch1 := make(chan int)
	ch2 := make(chan int)
	ch3 := make(chan int)
	ch4 := make(chan int)
	go func() {
		for {
			fmt.Println("I'm goroutine 1")
			time.Sleep(1 * time.Second)
			ch2 <- 1 //I'm done, you turn
			<-ch1
		}
	}()

	go func() {
		for {
			<-ch2
			fmt.Println("I'm goroutine 2")
			time.Sleep(1 * time.Second)
			ch3 <- 1
		}

	}()

	go func() {
		for {
			<-ch3
			fmt.Println("I'm goroutine 3")
			time.Sleep(1 * time.Second)
			ch4 <- 1
		}

	}()

	go func() {
		for {
			<-ch4
			fmt.Println("I'm goroutine 4")
			time.Sleep(1 * time.Second)
			ch1 <- 1
		}

	}()

	select {}
}

func TestTaskScheduling(t *testing.T) {

	ch := make(chan struct{})
	for i := 1; i <= 4; i++ {
		go func(index int) {
			time.Sleep(time.Duration(index*10) * time.Millisecond) //核心用时间控制顺序，不推荐
			for {
				<-ch
				fmt.Printf("I am No %d Goroutine\n", index)
				time.Sleep(time.Second)
				ch <- struct{}{}
			}
		}(i)
	}
	ch <- struct{}{}
	time.Sleep(time.Minute)

}
