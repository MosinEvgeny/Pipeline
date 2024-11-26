package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Настройки буферизации.
const (
	defaultBufferSize    = 5               // Размер буфера
	defaultFlushInterval = 5 * time.Second // Интервал очистки буфера
)

// RingBuffer - структура для кольцевого буфера.
type RingBuffer struct {
	data []int
	head int
	tail int
	size int
	mu   sync.Mutex
}

// NewRingBuffer - создание нового кольцевого буфера.
func NewRingBuffer(size int) *RingBuffer {
	log.Println("Создан новый кольцевой буфер размером:", size)
	return &RingBuffer{
		data: make([]int, size),
		size: size,
	}
}

// Push - добавление элемента в буфер.
func (rb *RingBuffer) Push(val int) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	log.Println("Добавлено значение в буфер:", val)
	rb.data[rb.tail] = val
	rb.tail = (rb.tail + 1) % rb.size
	if rb.tail == rb.head {
		rb.head = (rb.head + 1) % rb.size // Перезапись старых данных при переполнении
		log.Println("Буфер переполнен. Старые данные перезаписаны.")
	}
}

// Flush - получение всех элементов из буфера с очисткой.
func (rb *RingBuffer) Flush() []int {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.head == rb.tail {
		log.Println("Буфер пуст. Очистка не требуется.")
		return nil // Буфер пуст
	}
	log.Println("Очистка буфера.")
	data := make([]int, 0, rb.size)
	for rb.head != rb.tail {
		data = append(data, rb.data[rb.head])
		rb.head = (rb.head + 1) % rb.size
	}
	return data
}

// Стадия пайплайна: фильтр отрицательных чисел.
func filterNegative(in <-chan int, out chan<- int, done <-chan bool) {
	defer close(out)
	log.Println("Стадия filterNegative запущена.")
	for {
		select {
		case n := <-in:
			log.Println("Получено значение:", n)
			if n >= 0 {
				log.Println("Значение", n, "прошло фильтр (>= 0)")
				out <- n
			} else {
				log.Println("Значение", n, "не прошло фильтр (< 0)")
			}
		case <-done:
			log.Println("Стадия filterNegative завершена.")
			return
		}
	}
}

// Стадия пайплайна: фильтр чисел, не кратных 3 (исключая 0).
func filterNotDivisibleBy3(in <-chan int, out chan<- int, done <-chan bool) {
	defer close(out)
	log.Println("Стадия filterNotDivisibleBy3 запущена.")
	for {
		select {
		case n := <-in:
			log.Println("Получено значение:", n)
			if n != 0 && n%3 == 0 {
				log.Println("Значение", n, "прошло фильтр (!= 0 && кратно 3)")
				out <- n
			} else {
				log.Println("Значение", n, "не прошло фильтр")
			}
		case <-done:
			log.Println("Стадия filterNotDivisibleBy3 завершена.")
			return
		}
	}
}

// Стадия пайплайна: буферизация и периодическая отправка данных.
func bufferAndSend(in <-chan int, out chan<- int, done <-chan bool, bufferSize int, flushInterval time.Duration) {
	defer close(out)
	log.Println("Стадия bufferAndSend запущена.")
	buffer := NewRingBuffer(bufferSize)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case n := <-in:
			log.Println("Получено значение:", n)
			buffer.Push(n)
		case <-ticker.C:
			log.Println("Сработал таймер.")
			for _, n := range buffer.Flush() {
				out <- n
				log.Println("Отправлено значение из буфера:", n)
			}
		case <-done:
			log.Println("Стадия bufferAndSend завершена. Очистка буфера перед завершением.")
			// Очистка буфера перед завершением
			for _, n := range buffer.Flush() {
				out <- n
				log.Println("Отправлено значение из буфера:", n)
			}
			return
		}
	}
}

func main() {
	log.SetOutput(os.Stdout) // Направляем логи в консоль
	// Обработка прерывания
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// Канал для сигнала завершения работы
	done := make(chan bool)
	defer close(done)
	log.Println("Программа запущена.")

	fmt.Println("Программа запущена. Начинайте вводить целые числа:")

	// Источник данных: чтение чисел из консоли
	input := make(chan int)
	go func() {
		defer close(input)
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if num, err := strconv.Atoi(line); err == nil {
				log.Println("Введено число:", num)
				input <- num
			} else {
				log.Println("Некорректный ввод:", line)
				fmt.Println("Некорректный ввод. Введите целое число:")
			}
		}
		fmt.Println("Ввод завершен.")
		log.Println("Ввод завершен.")
	}()

	// Создание каналов для пайплайна
	stage1Out := make(chan int)
	stage2Out := make(chan int)
	pipelineOut := make(chan int)

	// Запуск стадий пайплайна
	go filterNegative(input, stage1Out, done)
	go filterNotDivisibleBy3(stage1Out, stage2Out, done)
	go bufferAndSend(stage2Out, pipelineOut, done, defaultBufferSize, defaultFlushInterval)

	fmt.Println("Обработанные данные:")

	// Вывод обработанных данных
	for {
		select {
		case num := <-pipelineOut:
			fmt.Printf("Получены данные: %d\n", num)
			log.Println("Выведены данные:", num)
		case <-interrupt:
			fmt.Println("\nПрограмма завершена по запросу пользователя.")
			log.Println("Программа завершена по запросу пользователя.")
			done <- true
			return
		}
	}
}
