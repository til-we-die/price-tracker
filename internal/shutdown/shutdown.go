// утилиты для корректного завершения работы
package shutdown

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type ShutdownManager struct {
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	shutdownCh   chan struct{}
	mu           sync.Mutex
	isShutting   bool
	timeout      time.Duration
	cleanupFuncs []func() error
}

func NewShutdownManager(timeout time.Duration) *ShutdownManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ShutdownManager{
		ctx:        ctx,
		cancel:     cancel,
		shutdownCh: make(chan struct{}),
		timeout:    timeout,
	}
}

func (sm *ShutdownManager) Context() context.Context {
	return sm.ctx
}

func (sm *ShutdownManager) AddCleanup(fn func() error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.cleanupFuncs = append(sm.cleanupFuncs, fn)
}

// увеличивает счетчик активных операций
func (sm *ShutdownManager) AddDelta(delta int) {
	sm.wg.Add(delta)
}

func (sm *ShutdownManager) Done() {
	sm.wg.Done()
}

func (sm *ShutdownManager) WaitGroup() *sync.WaitGroup {
	return &sm.wg
}

func (sm *ShutdownManager) IsShuttingDown() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.isShutting
}

func (sm *ShutdownManager) WaitForShutdown() {
	// Настройка обработки сигналов
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Ожидание сигнала
	sig := <-sigCh
	log.Printf("[INFO] received signal: %v, starting graceful shutdown...", sig)

	sm.cancel()

	sm.mu.Lock()
	sm.isShutting = true
	sm.mu.Unlock()

	done := make(chan struct{})
	go func() {
		sm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("[INFO] all tasks completed gracefully")
	case <-time.After(sm.timeout):
		log.Printf("[WARN] shutdown timeout (%v) exceeded, forcing exit", sm.timeout)
	}

	for i, cleanup := range sm.cleanupFuncs {
		if err := cleanup(); err != nil {
			log.Printf("[WARN] cleanup function %d failed: %v", i, err)
		}
	}

	log.Println("[INFO] shutdown complete")
	close(sm.shutdownCh)
}

func (sm *ShutdownManager) ShutdownComplete() <-chan struct{} {
	return sm.shutdownCh
}
