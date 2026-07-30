package accessible

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/atotto/clipboard"

	"github.com/matinsenpai/senpaiscanner/internal/config"
	"github.com/matinsenpai/senpaiscanner/internal/engine"
	"github.com/matinsenpai/senpaiscanner/internal/ipsrc"
	"github.com/matinsenpai/senpaiscanner/internal/prober"
	"github.com/matinsenpai/senpaiscanner/internal/result"
)

// Run starts a simple text-based scan designed for screen readers.
// It avoids the full-screen TUI and prints line-by-line progress and results.
func Run(version string) {
	fmt.Println("SenPai Scanner — Accessible Mode")
	fmt.Println("Version:", version)
	fmt.Println()
	fmt.Println("This mode is designed for screen readers such as NVDA and JAWS.")
	fmt.Println("All output is plain text, one line at a time.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// --- configuration prompts ---
	count := askInt(reader, "How many IPs to scan?", config.ScanDefaults.Count)
	concurrency := askInt(reader, "How many concurrent workers?", config.ScanDefaults.Concurrency)
	timeoutSec := askInt(reader, "Timeout in seconds?", int(config.ScanDefaults.Timeout.Seconds()))
	port := askInt(reader, "Port?", config.ScanDefaults.Port)
	topN := askInt(reader, "How many top results to show at the end?", config.ScanDefaults.Top)

	fmt.Println()
	fmt.Println("Starting scan...")
	fmt.Printf("Settings: count=%d, workers=%d, timeout=%ds, port=%d, mode=http\n", count, concurrency, timeoutSec, port)
	fmt.Println("Press Ctrl+C to stop early.")
	fmt.Println()

	src, err := ipsrc.New(true, false, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error loading IP ranges:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println()
		fmt.Println("Stopping scan...")
		cancel()
	}()

	eng := engine.New(engine.Config{
		Concurrency: concurrency,
		ProbeConfig: prober.Config{
			Port:               port,
			Mode:               prober.ModeHTTP,
			Tries:              config.ScanDefaults.Tries,
			Timeout:            time.Duration(timeoutSec) * time.Second,
			SpeedBytes:         64 * 1024,
			InsecureSkipVerify: true,
		},
	})

	ipStream := src.Stream(ctx, count)

	var (
		mu       sync.Mutex
		healthy  []*result.Result
		lastLog  time.Time
		logEvery = 2 * time.Second
	)

	start := time.Now()
	eng.Run(ctx, ipStream, func(r *result.Result) {
		if r.IsHealthy() {
			mu.Lock()
			healthy = append(healthy, r)
			mu.Unlock()
			fmt.Printf("FOUND healthy IP: %s  latency=%s  colo=%s  loss=%.0f%%\n",
				r.IP.String(), r.Avg().Round(time.Millisecond), r.Colo, r.Loss())
		}

		stats := eng.Stats()
		now := time.Now()
		if now.Sub(lastLog) >= logEvery {
			lastLog = now
			fmt.Printf("Progress: tested=%d  healthy=%d  failed=%d  in-flight=%d  elapsed=%s\n",
				stats.Tested.Load(),
				stats.Healthy.Load(),
				stats.Failed.Load(),
				stats.InFlight.Load(),
				time.Since(start).Round(time.Second),
			)
		}
	})

	fmt.Println()
	fmt.Println("Scan finished.")
	stats := eng.Stats()
	fmt.Printf("Total tested: %d\n", stats.Tested.Load())
	fmt.Printf("Healthy: %d\n", stats.Healthy.Load())
	fmt.Printf("Failed: %d\n", stats.Failed.Load())
	fmt.Printf("Elapsed: %s\n", time.Since(start).Round(time.Second))
	fmt.Println()

	mu.Lock()
	defer mu.Unlock()

	if len(healthy) == 0 {
		fmt.Println("No healthy IPs found.")
		return
	}

	// Sort by average latency
	sort.SliceStable(healthy, func(i, j int) bool {
		return healthy[i].Avg() < healthy[j].Avg()
	})

	if topN > 0 && topN < len(healthy) {
		healthy = healthy[:topN]
	}

	fmt.Println("Top healthy IPs (sorted by latency):")
	fmt.Println("------------------------------------")
	for i, r := range healthy {
		fmt.Printf("%d. %s  latency=%s  colo=%s  loss=%.0f%%\n",
			i+1, r.IP.String(), r.Avg().Round(time.Millisecond), r.Colo, r.Loss())
	}
	fmt.Println()

	// Save to file
	outFile := "accessible-results.txt"
	f, err := os.Create(outFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not create results file:", err)
	} else {
		defer f.Close()
		for _, r := range healthy {
			fmt.Fprintln(f, r.IP.String())
		}
		fmt.Println("Results saved to:", outFile)
	}

	// Copy first IP to clipboard (optional convenience)
	if len(healthy) > 0 {
		if err := clipboard.WriteAll(healthy[0].IP.String()); err == nil {
			fmt.Println("Fastest IP copied to clipboard:", healthy[0].IP.String())
		}
	}

	fmt.Println()
	fmt.Println("Done. You can close this window.")
}

func askInt(reader *bufio.Reader, prompt string, defaultVal int) int {
	fmt.Printf("%s [%d]: ", prompt, defaultVal)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(line)
	if err != nil || n <= 0 {
		fmt.Println("Invalid number, using default:", defaultVal)
		return defaultVal
	}
	return n
}
