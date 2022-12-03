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
	client    http.Client
	pipeline  chan QueueHandler
	waitGroup sync.WaitGroup
}

type QueueHandler interface {
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
	resp, err := p.client.Do(r)
	if err == nil && accept != nil {
		accept(resp)
	}
}

// Run waits for channel messages and handles them.
// Expected to be run in a goroutine.
func (p *OutputPipeline) Run(ctx context.Context) error {
	telemetry.Trace("running output pipeline")
	// Wait for context end or messages from the pipeline channel
	select {
	// case <-p.stop:
	// 	return nil
	case <-ctx.Done():
		telemetry.Log("pipeline cancelled: %s", ctx.Err())
		return ctx.Err()
	case handler := <-p.pipeline:
		telemetry.Trace("pipeline queue, message received")
		r, err := handler.Prepare(p)
		if err != nil {
			telemetry.Error(err, "pipeline queue, getting request")
		} else {
			resp, err := p.client.Do(r)
			if err != nil {
				telemetry.Error(err, "pipeline queue, getting response")
			} else {
				handler.Receive(resp)
			}
			p.waitGroup.Done()
		}
	}
	return nil
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

func (s *OutputPipeline) ActivityPostRequest(url string, v any) (*http.Request, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshaling json from object: %w", err)
	}
	reader := bytes.NewBuffer(body)
	r, err := http.NewRequest(http.MethodPost, url, reader)
	if err != nil {
		return nil, fmt.Errorf("creating ActivityPub request: %w", err)
	}
	r.Header.Set("Accept", activity.ContentType)
	return r, nil
}

// LookupActor finds the remote endpoint for the actor ID, which is assumed to be a URL
// Blocks until we get a response or the context is cancelled or times out
func (s *OutputPipeline) LookupActor(ctx context.Context, id string) (*activity.Actor, error) {
	telemetry.Trace("Looking up actor %s", id)

	var actor activity.Actor
	r, err := http.NewRequest(http.MethodGet, id, nil)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Accept", activity.ContentType) // make sure we get a json response

	// TODO: maybe support webfingering an acct:x@y resource too
	// TODO: make this more asynchronous, and (optionally?) cache the results locally
	// TODO: retry periodically

	// TODO: My pipeline isn't working with channels, gets caught in race conditions... fix that.
	// Although for this particular function a synchronous call and response is okay.
	var respErr error
	s.SendAndWait(r, func(resp *http.Response) {
		// On getting a response...
		jsonBytes, err := io.ReadAll(io.LimitReader(resp.Body, 4000))
		if err != nil {
			respErr = fmt.Errorf("reading response bytes: %w", err)
			return
		}
		telemetry.Trace("got response from actor %s", string(jsonBytes))
		respErr = json.Unmarshal(jsonBytes, &actor)
	})

	if respErr != nil {
		telemetry.Error(err, "looking up user ID [%s]", id)
		return nil, respErr
	}
	return &actor, nil
}
