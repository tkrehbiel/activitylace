package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/telemetry"
)

// OutputPipeline is intended to be an asychronous rate-limited output pipeline for sending http requests.
// The idea is to be able to queue up a large number of requests to send staggered over time, rather than all at once.
// (The rate limiting is not yet implemented.)
type OutputPipeline struct {
	host      string
	client    http.Client
	pipeline  chan QueueHandler
	waitGroup sync.WaitGroup
}

type QueueHandler interface {
	fmt.Stringer
	Prepare(*OutputPipeline) (*http.Request, error)
	Receive(resp *http.Response)
}

func (p *OutputPipeline) Queue(handler QueueHandler) {
	if p == nil {
		panic("no pipeline")
	}
	if p.pipeline == nil {
		panic("no pipeline channel")
	}
	p.waitGroup.Add(1)
	p.pipeline <- handler
}

// Flush blocks until the pipeline is empty
func (p *OutputPipeline) Flush() {
	p.waitGroup.Wait()
}

func (p *OutputPipeline) SendAndWait(r *http.Request, accept func(resp *http.Response)) {
	telemetry.Request(r, "outgoing")
	resp, err := p.client.Do(r)
	if err == nil && accept != nil {
		telemetry.Response(resp, "%s", r.URL)
		accept(resp)
	}
}

// Run waits for channel messages and handles them.
// Expected to be run in a goroutine.
func (p *OutputPipeline) Run(ctx context.Context) error {
	telemetry.Trace("running output pipeline")
	// Wait for context end or messages from the pipeline channel
	for {
		select {
		// case <-p.stop:
		// 	return nil
		case <-ctx.Done():
			telemetry.Log("pipeline cancelled: %s", ctx.Err())
			return ctx.Err()
		case handler := <-p.pipeline:
			telemetry.Trace("pipeline queue, message received [%s]", handler.String())
			r, err := handler.Prepare(p)
			if err != nil {
				telemetry.Error(err, "pipeline queue, getting request")
			} else {
				telemetry.Request(r, "outgoing")
				resp, err := p.client.Do(r)
				if err != nil {
					telemetry.Error(err, "pipeline queue, getting response")
				} else {
					telemetry.Response(resp, "%s", r.URL)
					handler.Receive(resp)
				}
				p.waitGroup.Done()
			}
		}
	}
}

func (p *OutputPipeline) Stop() {
	p.Flush()
}

func NewPipeline() *OutputPipeline {
	return &OutputPipeline{
		client: http.Client{
			Timeout: time.Second * 5,
		},
		pipeline: make(chan QueueHandler),
	}
}

func (s *OutputPipeline) ActivityRequest(method string, url string, v any) (*http.Request, error) {
	// TODO: Make this a global function that doesn't require a pipeline
	var reader io.Reader
	if v != nil {
		body, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshaling json from object: %w", err)
		}
		telemetry.Trace("creating request body %s", string(body))
		reader = bytes.NewBuffer(body)
	}
	r, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("creating ActivityPub request: %w", err)
	}
	r.Header.Set("User-Agent", "Activitylace/0.1 (+https://github.com/tkrehbiel/activitylace)")
	r.Header.Set("Content-Type", activity.ContentType)
	r.Header.Set("Accept", activity.ContentType)
	r.Header.Set("Host", s.host)
	r.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	return r, nil
}
