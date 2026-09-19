package metrics

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type Registry struct {
	serviceName   string
	startTime     time.Time
	requestsTotal uint64
	statusCodes   map[int]*uint64
	mu            sync.RWMutex
}

var (
	defaultRegistry *Registry
	once            sync.Once
)

func Init(serviceName string) *Registry {
	once.Do(func() {
		defaultRegistry = &Registry{
			serviceName: serviceName,
			startTime:   time.Now().UTC(),
			statusCodes: make(map[int]*uint64),
		}
	})
	return defaultRegistry
}

func GetRegistry() *Registry {
	if defaultRegistry == nil {
		return Init("aarot-service")
	}
	return defaultRegistry
}

func (r *Registry) RecordRequest(statusCode int) {
	atomic.AddUint64(&r.requestsTotal, 1)

	r.mu.Lock()
	counter, exists := r.statusCodes[statusCode]
	if !exists {
		var val uint64
		r.statusCodes[statusCode] = &val
		counter = &val
	}
	r.mu.Unlock()

	atomic.AddUint64(counter, 1)
}

func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		uptime := time.Since(r.startTime).Seconds()
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		fmt.Fprintf(w, "# HELP http_requests_total Total number of HTTP requests processed.\n")
		fmt.Fprintf(w, "# TYPE http_requests_total counter\n")
		fmt.Fprintf(w, "http_requests_total{service=\"%s\"} %d\n\n", r.serviceName, atomic.LoadUint64(&r.requestsTotal))

		fmt.Fprintf(w, "# HELP http_requests_by_status_total HTTP requests partitioned by status code.\n")
		fmt.Fprintf(w, "# TYPE http_requests_by_status_total counter\n")
		r.mu.RLock()
		for code, counter := range r.statusCodes {
			fmt.Fprintf(w, "http_requests_by_status_total{service=\"%s\",code=\"%d\"} %d\n", r.serviceName, code, atomic.LoadUint64(counter))
		}
		r.mu.RUnlock()

		fmt.Fprintf(w, "\n# HELP process_uptime_seconds Process uptime in seconds.\n")
		fmt.Fprintf(w, "# TYPE process_uptime_seconds gauge\n")
		fmt.Fprintf(w, "process_uptime_seconds{service=\"%s\"} %.2f\n\n", r.serviceName, uptime)

		fmt.Fprintf(w, "# HELP go_goroutines Number of active goroutines.\n")
		fmt.Fprintf(w, "# TYPE go_goroutines gauge\n")
		fmt.Fprintf(w, "go_goroutines{service=\"%s\"} %d\n\n", r.serviceName, runtime.NumGoroutine())

		fmt.Fprintf(w, "# HELP go_memstats_alloc_bytes Number of bytes allocated and still in use.\n")
		fmt.Fprintf(w, "# TYPE go_memstats_alloc_bytes gauge\n")
		fmt.Fprintf(w, "go_memstats_alloc_bytes{service=\"%s\"} %d\n", r.serviceName, memStats.Alloc)
	}
}
