package buildqueue

import (
	"context"
	"log"
	"sync"
)

type Job struct {
	AppID string
	Fn    func(ctx context.Context) error
}

type Queue struct {
	ch     chan Job
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func New(bufSize int) *Queue {
	return &Queue{
		ch: make(chan Job, bufSize),
	}
}

func (q *Queue) Start(ctx context.Context) {
	ctx, q.cancel = context.WithCancel(ctx)
	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		for {
			select {
			case job, ok := <-q.ch:
				if !ok {
					return
				}
				log.Printf("buildqueue: starting job for %s", job.AppID)
				if err := job.Fn(ctx); err != nil {
					log.Printf("buildqueue: job %s failed: %v", job.AppID, err)
				} else {
					log.Printf("buildqueue: job %s completed", job.AppID)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (q *Queue) Enqueue(job Job) bool {
	select {
	case q.ch <- job:
		return true
	default:
		return false
	}
}

func (q *Queue) Stop() {
	if q.cancel != nil {
		q.cancel()
	}
	close(q.ch)
	q.wg.Wait()
}
