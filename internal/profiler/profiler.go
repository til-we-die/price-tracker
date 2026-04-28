package profiler

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"runtime"
	"runtime/pprof"
	"time"
)

func StartPprof(port string) {
	go func() {
		log.Printf("[INFO] pprof server starting on http://localhost%s/debug/pprof/", port)
		if err := http.ListenAndServe(port, nil); err != nil {
			log.Printf("[WARN] pprof server failed: %v", err)
		}
	}()
}

func PrintMemStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("[DEBUG] MemStats: Alloc=%dMB, TotalAlloc=%dMB, Sys=%dMB, NumGC=%d, Goroutines=%d",
		m.Alloc/1024/1024,
		m.TotalAlloc/1024/1024,
		m.Sys/1024/1024,
		m.NumGC,
		runtime.NumGoroutine(),
	)
}

func StartPeriodicMemStats(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				PrintMemStats()
			case <-stop:
				ticker.Stop()
				return
			}
		}
	}()
}

type ProfileConfig struct {
	CPUProfileFile     string
	MemProfileFile     string
	BlockProfileFile   string
	MutexProfileFile   string
	GoroutineFile      string
	CPUProfileDuration time.Duration
}

func SaveProfiles(config ProfileConfig) error {
	if err := os.MkdirAll("profiles", 0755); err != nil {
		return err
	}

	if config.CPUProfileFile != "" {
		f, err := os.Create("profiles/" + config.CPUProfileFile)
		if err != nil {
			return err
		}
		defer f.Close()

		log.Printf("[INFO] collecting CPU profile for %v...", config.CPUProfileDuration)
		pprof.StartCPUProfile(f)
		time.Sleep(config.CPUProfileDuration)
		pprof.StopCPUProfile()
		log.Printf("[INFO] CPU profile saved to profiles/%s", config.CPUProfileFile)
	}

	if config.MemProfileFile != "" {
		f, err := os.Create("profiles/" + config.MemProfileFile)
		if err != nil {
			return err
		}
		defer f.Close()

		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			return err
		}
		log.Printf("[INFO] memory profile saved to profiles/%s", config.MemProfileFile)
	}

	if config.BlockProfileFile != "" {
		runtime.SetBlockProfileRate(1)
		defer runtime.SetBlockProfileRate(0)

		f, err := os.Create("profiles/" + config.BlockProfileFile)
		if err != nil {
			return err
		}
		defer f.Close()

		if err := pprof.Lookup("block").WriteTo(f, 0); err != nil {
			return err
		}
		log.Printf("[INFO] block profile saved to profiles/%s", config.BlockProfileFile)
	}

	// Mutex профиль
	if config.MutexProfileFile != "" {
		runtime.SetMutexProfileFraction(1)
		defer runtime.SetMutexProfileFraction(0)

		f, err := os.Create("profiles/" + config.MutexProfileFile)
		if err != nil {
			return err
		}
		defer f.Close()

		if err := pprof.Lookup("mutex").WriteTo(f, 0); err != nil {
			return err
		}
		log.Printf("[INFO] mutex profile saved to profiles/%s", config.MutexProfileFile)
	}

	// Goroutine профиль
	if config.GoroutineFile != "" {
		f, err := os.Create("profiles/" + config.GoroutineFile)
		if err != nil {
			return err
		}
		defer f.Close()

		if err := pprof.Lookup("goroutine").WriteTo(f, 0); err != nil {
			return err
		}
		log.Printf("[INFO] goroutine profile saved to profiles/%s", config.GoroutineFile)
	}

	return nil
}

func ViewProfileInBrowser(profileFile string) error {
	cmd := exec.Command("go", "tool", "pprof", "-http=:8080", "profiles/"+profileFile)
	log.Printf("[INFO] opening profile %s in browser...", profileFile)
	return cmd.Start()
}
