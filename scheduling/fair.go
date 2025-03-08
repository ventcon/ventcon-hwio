package scheduling

import (
	"reflect"

	"github.com/ansel1/merry/v2"
)

type fairScheduler[T any] struct {
	sources []<-chan T
	sink    chan<- T
	stop    chan bool
}

func NewFairScheduler[T any](sink chan<- T) Scheduler[T] {
	return &fairScheduler[T]{
		sources: make([]<-chan T, 0),
		sink:    sink,
	}
}

func (scheduler *fairScheduler[T]) AddSource(source <-chan T) error {
	if scheduler.stop != nil {
		return merry.New("Cannot add sources after starting the scheduler.")
	}
	scheduler.sources = append(scheduler.sources, source)
	return nil
}

func (scheduler *fairScheduler[T]) Start() {
	if len(scheduler.sources) == 0 {
		close(scheduler.sink)
		return
	}
	scheduler.stop = make(chan bool)
	go scheduler.run()
}

func (scheduler *fairScheduler[T]) run() {
	sourceClosed := make([]bool, len(scheduler.sources))
	sourceRatelimited := make([]bool, len(scheduler.sources))

	cases := make([]reflect.SelectCase, len(scheduler.sources)+2)

	for i, source := range scheduler.sources {
		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(source),
		}
	}

	// Stop signal
	stopChannelIndex := len(cases) - 2
	cases[stopChannelIndex] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(scheduler.stop),
	}
	// Default case
	defaultCaseIndex := len(cases) - 1
	cases[defaultCaseIndex] = reflect.SelectCase{
		Dir:  reflect.SelectDefault,
		Chan: reflect.ValueOf(nil),
	}

	for {
		chosen, value, ok := reflect.Select(cases)
		if chosen == stopChannelIndex {
			// Stop signal
			close(scheduler.sink)
			return
		} else if chosen == defaultCaseIndex {
			// Default case: no source is ready
			// Reset all ratelimits
			for i, source := range scheduler.sources {
				if !sourceClosed[i] {
					if sourceRatelimited[i] {
						cases[i].Chan = reflect.ValueOf(source)
						sourceRatelimited[i] = false
					}
				}
			}
			// To avoid busy-waiting, disable the default case until we have a source ready
			cases[defaultCaseIndex].Dir = reflect.SelectRecv
		} else if !ok {
			// Source is closed
			sourceClosed[chosen] = true
			cases[chosen].Chan = reflect.ValueOf(nil) // Remove the channel from the select
			allClosed := true
			for _, closed := range sourceClosed {
				if !closed {
					allClosed = false
					break
				}
			}
			if allClosed {
				close(scheduler.sink)
				return
			}
		} else {
			// Ratelimit the source
			sourceRatelimited[chosen] = true
			cases[chosen].Chan = reflect.ValueOf(nil) // Remove the channel from the select for now
			// Re-enable the default case in order to not wait forever if no source is ready
			cases[defaultCaseIndex].Dir = reflect.SelectDefault

			// Send the value to the sink
			scheduler.sink <- value.Interface().(T)
		}
	}
}

func (scheduler *fairScheduler[T]) Stop() {
	if scheduler.stop != nil {
		close(scheduler.stop)
	}
}
