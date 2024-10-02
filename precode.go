package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

func Generator(ctx context.Context, ch chan<- int64, fn func(int64)) {
	defer close(ch) // Ensure the channel is closed when the function exits

	var n int64 = 1
	for {
		select {
		case <-ctx.Done():
			return // Exit if the context is done
		default:
			fn(n)   // Call the provided function with the current number
			ch <- n // Send the number to the channel
			n++     // Increment the number
		}
	}
}

func Worker(in <-chan int64, out chan<- int64) {
	defer close(out) // Ensure the output channel is closed when the function exits

	for {
		v, ok := <-in
		if !ok {
			return // Exit if the input channel is closed
		}
		out <- v                         // Send the number to the output channel
		time.Sleep(1 * time.Millisecond) // Pause for 1 millisecond
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel() // Ensure the context is canceled

	chIn := make(chan int64)

	var inputSum int64
	var inputCount int64

	go Generator(ctx, chIn, func(i int64) {
		atomic.AddInt64(&inputSum, i)   // Safely add to inputSum
		atomic.AddInt64(&inputCount, 1) // Safely increment inputCount
	})

	const NumOut = 5

	outs := make([]chan int64, NumOut)
	for i := 0; i < NumOut; i++ {
		outs[i] = make(chan int64)
		go Worker(chIn, outs[i])
	}

	amounts := make([]int64, NumOut)
	chOut := make(chan int64, NumOut)

	var wg sync.WaitGroup

	// Collect results from each worker
	for i := 0; i < NumOut; i++ {
		wg.Add(1)
		go func(in <-chan int64, index int) {
			defer wg.Done()
			for v := range in {
				amounts[index]++
				chOut <- v // Send the number to the output channel
			}
		}(outs[i], i)
	}

	// Close chOut when all workers are done
	go func() {
		wg.Wait()
		close(chOut)
	}()

	var count int64
	var sum int64

	// Read from the output channel
	for v := range chOut {
		count++
		sum += v
	}

	fmt.Println("Количество чисел", inputCount, count)
	fmt.Println("Сумма чисел", inputSum, sum)
	fmt.Println("Разбивка по каналам", amounts)

	if inputSum != sum {
		log.Fatalf("Ошибка: суммы чисел не равны: %d != %d\n", inputSum, sum)
	}
	if inputCount != count {
		log.Fatalf("Ошибка: количество чисел не равно: %d != %d\n", inputCount, count)
	}
	for _, v := range amounts {
		inputCount -= v
	}
	if inputCount != 0 {
		log.Fatalf("Ошибка: разделение чисел по каналам неверное\n")
	}
}
