package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

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
	// TODO: add rate limiting
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
