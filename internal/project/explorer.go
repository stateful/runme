package project

import (
	"context"
	"sync"
)

type Explorer struct {
	ctx context.Context
	p   *Project

	once    sync.Once
	events  []LoadEvent
	loadErr error
}

func NewExplorer(ctx context.Context, p *Project) *Explorer {
	return &Explorer{
		ctx:  ctx,
		p:    p,
		once: sync.Once{},
	}
}

func (e *Explorer) load() {
	e.once.Do(func() {
		eventc := make(chan LoadEvent)
		events := make([]LoadEvent, 0)
		done := make(chan struct{})

		var loadErr error

		go func() {
			defer close(done)
			for event := range eventc {
				if loadErr == nil && event.Type == LoadEventError {
					loadErr = event.Data.(error)
				}
				events = append(events, event)
			}
		}()

		e.p.Load(e.ctx, eventc, false)
		<-done

		e.events = events
		e.loadErr = loadErr
	})
}

func (e *Explorer) Files() ([]string, error) {
	e.load()

	if e.loadErr != nil {
		return nil, e.loadErr
	}

	files := filter(e.events, func(le LoadEvent) bool { return le.Type == LoadEventFoundFile })
	names := mapp(files, func(le LoadEvent) string {
		return le.Data.(string)
	})

	return names, nil
}

func (e *Explorer) Tasks() ([]CodeBlock, error) {
	e.load()

	if e.loadErr != nil {
		return nil, e.loadErr
	}

	taskEvents := filter(e.events, func(le LoadEvent) bool { return le.Type == LoadEventFoundTask })
	tasks := mapp(taskEvents, func(le LoadEvent) CodeBlock { return le.Data.(CodeBlock) })

	return tasks, nil
}

func filter[T any, S ~[]T](s S, fn func(T) bool) S {
	result := make(S, 0, len(s))

	for _, item := range s {
		if fn(item) {
			result = append(result, item)
		}
	}

	return result
}

func mapp[T any, R any, S ~[]T](s S, fn func(T) R) []R {
	result := make([]R, 0, len(s))

	for _, item := range s {
		result = append(result, fn(item))
	}

	return result
}
