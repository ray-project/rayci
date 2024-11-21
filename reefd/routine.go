package main

import (
	"fmt"
	"time"
)

func count(i int, ch chan int) {
	fmt.Println("counting", i)
	ch <- i
}
func main() {
	// make a channel
	ch := make(chan int)
	go count(1, ch)
	go count(2, ch)
	go count(3, ch)
	go count(4, ch)
	time.Sleep(3 * time.Second)
	for i := 0; i < 4; i++ {
		fmt.Println(<-ch)
	}
}
