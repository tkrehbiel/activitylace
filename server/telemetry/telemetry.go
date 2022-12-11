// A simple telemetry package.
// As yet we have no place to put counters except log messages.
package telemetry

import (
	"bytes"
	"fmt"
	"io/ioutil"
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
	if err != nil {
		data.logger.Println("ERROR", fmt.Sprintf(format, args...), fmt.Sprintf("[%s]", err))
	} else {
		data.logger.Println("ERROR", fmt.Sprintf(format, args...))
	}
	Increment("errors", 1)
}

// Request logs essential information about an HTTP request
func Request(r *http.Request, format string, args ...any) {
	lines := make([]string, 0)
	lines = append(lines, fmt.Sprintf("%s %s", r.Method, r.URL.String()))
	for k, v := range r.Header {
		s := fmt.Sprintf("%s: %s", k, strings.Join(v, ", "))
		lines = append(lines, s)
	}
	hdr := strings.Join(lines, "\n")

	var body string
	if r.Body != nil {
		buf, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			body = "(error reading body)"
		} else if len(buf) > 0 {
			body = string(buf)
		} else {
			body = "(no body)"
		}
		reader := ioutil.NopCloser(bytes.NewBuffer(buf))
		r.Body = reader
	} else {
		body = "(no body)"
	}

	Trace("request %s: %s\n%s", fmt.Sprintf(format, args...), hdr, body)
}

// Response logs essential information about an HTTP response
func Response(resp *http.Response, format string, args ...any) {
	headers := make([]string, 0)
	for k, v := range resp.Header {
		s := fmt.Sprintf("%s: %s", k, strings.Join(v, ", "))
		headers = append(headers, s)
	}
	s := strings.Join(headers, " | ")

	if resp.Body != nil {
		buf, err := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		if err != nil {
			s += ", error reading body"
		} else if len(buf) > 0 {
			s += fmt.Sprintf(", body:%s", strings.TrimSpace(string(buf)))
		} else {
			s += ", no body"
		}
		reader := ioutil.NopCloser(bytes.NewBuffer(buf))
		resp.Body = reader
	} else {
		s += ", no body"
	}

	Trace("response %s, status: %d, headers: %s", fmt.Sprintf(format, args...), resp.StatusCode, s)
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
