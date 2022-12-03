package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/telemetry"
)

// OutputPipeline is intended to be an asychronous rate-limited output pipeline for sending http requests.
// The idea is to be able to queue up a large number of requests to send staggered over time, rather than all at once.
// (The rate limiting is not yet implemented.)
type OutputPipeline struct {
	client   http.Client
	pipeline chan AsyncRequest
	stop     chan bool
}

// AsyncRequest is an asynchronous request and a handler for the response
type AsyncRequest struct {
	Request *http.Request
	Handler func(resp *http.Response)
}

func (p *OutputPipeline) Send(r *http.Request, accept func(resp *http.Response)) {
	if p == nil {
		panic("no pipeline")
	}
	if p.pipeline == nil {
		panic("no pipeline channel")
	}
	p.pipeline <- AsyncRequest{Request: r, Handler: accept}
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
	// Wait for context end or messages from the pipeline channel
	select {
	// case <-p.stop:
	// 	return nil
	case <-ctx.Done():
		return ctx.Err()
	case msg := <-p.pipeline:
		resp, err := p.client.Do(msg.Request)
		if err == nil && msg.Handler != nil {
			msg.Handler(resp)
		}
	}
	return nil
}

func (p *OutputPipeline) Stop() {
	//p.stop <- true
}

func NewPipeline() *OutputPipeline {
	return &OutputPipeline{
		client: http.Client{
			Timeout: time.Second * 5,
		},
		pipeline: make(chan AsyncRequest),
		stop:     make(chan bool),
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

	response := make(chan error)
	s.Send(r, func(resp *http.Response) {
		// On getting a response...
		jsonBytes, err := io.ReadAll(io.LimitReader(resp.Body, 4000))
		if err != nil {
			response <- fmt.Errorf("reading response bytes: %w", err)
			return
		}
		telemetry.Trace("got response from actor %s", string(jsonBytes))
		if err := json.Unmarshal(jsonBytes, &actor); err != nil {
			response <- fmt.Errorf("unmarshaling json [%s]: %w", string(jsonBytes), err)
			return
		}
		response <- nil // just says we're done without error
	})

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*10) // TODO: configurable time
	defer cancel()

	select {
	case respErr := <-response:
		if respErr != nil {
			telemetry.Error(err, "looking up user ID [%s]", id)
			return nil, respErr
		}
		return &actor, nil
	case <-timeoutCtx.Done():
		return nil, ctx.Err()
	}
}
