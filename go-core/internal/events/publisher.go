// Package events fan-outs domain events (swipe, match, ...) from the Go core
// into a Redis Stream. python-analytics reads the same stream as a consumer
// group and persists the events into ClickHouse — this is the bridge between
// the two services described in the project README.
package events

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Event struct {
	Type    string
	Payload map[string]any
}

// Publisher is a small worker-pool: Publish() never blocks the HTTP handler
// that calls it, it just drops the event into a buffered channel. A fixed
// number of goroutines drains that channel concurrently and does the actual
// (slower) network call to Redis.
type Publisher struct {
	client *redis.Client
	stream string
	queue  chan Event
	wg     sync.WaitGroup
}

func NewPublisher(client *redis.Client, stream string, workers, bufferSize int) *Publisher {
	return &Publisher{
		client: client,
		stream: stream,
		queue:  make(chan Event, bufferSize),
	}
}

// Start spawns the worker goroutines. Call once, before the first Publish.
func (p *Publisher) Start(ctx context.Context, workers int) {
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx)
	}
}

func (p *Publisher) worker(ctx context.Context) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-p.queue:
			if !ok {
				return
			}
			p.send(ctx, ev)
		}
	}
}

func (p *Publisher) send(ctx context.Context, ev Event) {
	values := map[string]any{"type": ev.Type}
	for k, v := range ev.Payload {
		values[k] = v
	}

	sendCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := p.client.XAdd(sendCtx, &redis.XAddArgs{
		Stream: p.stream,
		Values: values,
	}).Err(); err != nil {
		log.Printf("events: failed to publish %q: %v", ev.Type, err)
	}
}

// Publish is non-blocking: if the buffer is full the event is dropped and
// logged rather than slowing down the request that triggered it. Analytics
// events are best-effort, not part of the core transaction guarantees.
func (p *Publisher) Publish(ev Event) {
	select {
	case p.queue <- ev:
	default:
		log.Printf("events: queue full, dropping event %q", ev.Type)
	}
}

// Close stops accepting new events and waits for in-flight sends to finish.
func (p *Publisher) Close() {
	close(p.queue)
	p.wg.Wait()
}
