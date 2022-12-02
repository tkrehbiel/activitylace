// A simple telemetry package.
// As yet we have no place to put counters except log messages.
package telemetry

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type TelemetryData struct {
	logger Logger

	counterLock sync.Mutex
	counters    map[string]int

	trace bool
}

type Logger interface {
	Println(v ...any)
}

var data = TelemetryData{
	counters: make(map[string]int),
	trace:    true,
}

// init is called at program startup time to initialize the logger
func init() {
	l := log.New(os.Stdout, "", 0)
	l.SetOutput(formattedWriter{})
	data.logger = l
}

type formattedWriter struct{}

func (w formattedWriter) Write(bytes []byte) (int, error) {
	return fmt.Print(time.Now().UTC().Format("2006-01-02 15:04:05") + " " + string(bytes))
}

func Log(format string, args ...any) {
	data.logger.Println(fmt.Sprintf(format, args...))
}

func Trace(format string, args ...any) {
	if data.trace {
		Log(format, args...)
	}
}

func Error(err error, format string, args ...any) {
	data.logger.Println("ERROR", fmt.Sprintf(format, args...), fmt.Sprintf("[%s]", err))
	Increment("errors", 1)
}

// Request logs essential information about an HTTP request
func Request(r *http.Request, format string, args ...any) {
	data.logger.Println(fmt.Sprintf(format, args...), r.Method, r.URL)
}

// Increment increases a count, thread-safe
func Increment(name string, n int) {
	data.counterLock.Lock()
	defer data.counterLock.Unlock()
	data.counters[name] += n
}

func GetCounter(name string) int {
	data.counterLock.Lock()
	defer data.counterLock.Unlock()
	return data.counters[name]
}

func LogCounters() {
	s := make([]string, 0)
	data.counterLock.Lock()
	for k, v := range data.counters {
		s = append(s, fmt.Sprintf("%s=%d", k, v))
	}
	data.counterLock.Unlock()
	if len(s) == 0 {
		s = append(s, "no counters were recorded")
	}
	Log(strings.Join(s, ", "))
}
