package scheduling

import (
	"testing"
	"time"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestNewFairScheduler(t *testing.T) {
	sink := make(chan int)
	scheduler := NewFairScheduler(sink)
	test.NotNil(t, scheduler)

	fairScheduler, ok := scheduler.(*fairScheduler[int])
	if !ok {
		t.Error("Returned scheduler is not a fair scheduler")
	}
	test.Eq(t, sink, fairScheduler.sink)
	test.NotNil(t, fairScheduler.sources)
	test.Len(t, 0, fairScheduler.sources)
	test.Nil(t, fairScheduler.stop)
}

func TestAddSource(t *testing.T) {
	sink := make(chan int)
	scheduler := NewFairScheduler(sink)
	test.NotNil(t, scheduler)

	fairScheduler, ok := scheduler.(*fairScheduler[int])
	if !ok {
		t.Error("Returned scheduler is not a fair scheduler")
	}

	source := make(chan int)
	err := scheduler.AddSource(source)
	test.NoError(t, err)

	test.Len(t, 1, fairScheduler.sources)
	test.Eq(t, source, fairScheduler.sources[0])

	source2 := make(chan int)
	err = scheduler.AddSource(source2)
	test.NoError(t, err)

	test.Len(t, 2, fairScheduler.sources)
	test.Eq(t, source2, fairScheduler.sources[1])

	close(source)
	close(source2)
}

func TestAddSourceAfterStart(t *testing.T) {
	sink := make(chan int)
	scheduler := NewFairScheduler(sink)
	test.NotNil(t, scheduler)

	source := make(chan int)
	err := scheduler.AddSource(source)
	must.NoError(t, err)

	scheduler.Start()

	source2 := make(chan int)
	err = scheduler.AddSource(source2)
	test.Error(t, err)

	scheduler.Stop()
	close(source)
	close(source2)
}

func isChannelClose[K any](ch <-chan K) bool {
	select {
	case _, ok := <-ch:
		return !ok
	default:
		return false
	}
}

func TestStartWithoutSources(t *testing.T) {
	sink := make(chan int)
	scheduler := NewFairScheduler(sink)
	test.NotNil(t, scheduler)

	fairScheduler, ok := scheduler.(*fairScheduler[int])
	if !ok {
		t.Error("Returned scheduler is not a fair scheduler")
	}

	scheduler.Start()

	test.True(t, isChannelClose(sink))
	test.Nil(t, fairScheduler.stop)
}

func TestStartOneSource(t *testing.T) {
	sink := make(chan int)
	scheduler := NewFairScheduler(sink)
	test.NotNil(t, scheduler)

	source := make(chan int)
	err := scheduler.AddSource(source)
	must.NoError(t, err)

	scheduler.Start()

	go func() {
		source <- 42
		source <- 43
		source <- 44
	}()

	test.Eq(t, 42, <-sink)
	test.Eq(t, 43, <-sink)
	test.Eq(t, 44, <-sink)

	scheduler.Stop()
	close(source)
}

func TestStartEmptySourceDoesNotBlock(t *testing.T) {
	sink := make(chan int)
	scheduler := NewFairScheduler(sink)
	test.NotNil(t, scheduler)

	source1 := make(chan int)
	err := scheduler.AddSource(source1)
	must.NoError(t, err)

	source2 := make(chan int)
	err = scheduler.AddSource(source2)
	must.NoError(t, err)

	scheduler.Start()

	go func() {
		source1 <- 42
		source1 <- 43
		source1 <- 44
	}()

	test.Eq(t, 42, <-sink)
	test.Eq(t, 43, <-sink)
	test.Eq(t, 44, <-sink)

	scheduler.Stop()
	close(source1)
	close(source2)
}

func TestStartClosingAllSourcesCloseSink(t *testing.T) {
	sink := make(chan int)
	scheduler := NewFairScheduler(sink)
	test.NotNil(t, scheduler)

	source1 := make(chan int)
	err := scheduler.AddSource(source1)
	must.NoError(t, err)

	source2 := make(chan int)
	err = scheduler.AddSource(source2)
	must.NoError(t, err)

	scheduler.Start()

	go func() {
		source1 <- 42
		source2 <- 43
		source1 <- 44
		close(source1)
		close(source2)
	}()

	test.Eq(t, 42, <-sink)
	test.Eq(t, 43, <-sink)
	test.Eq(t, 44, <-sink)
	_, ok := <-sink
	test.False(t, ok)
}

func AbsDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

func TestStartNoSourceGetsAhead(t *testing.T) {
	num_messages := 50

	sink := make(chan int)
	scheduler := NewFairScheduler(sink)
	test.NotNil(t, scheduler)

	source1 := make(chan int)
	err := scheduler.AddSource(source1)
	must.NoError(t, err)

	source2 := make(chan int)
	err = scheduler.AddSource(source2)
	must.NoError(t, err)

	source3 := make(chan int)
	err = scheduler.AddSource(source3)
	must.NoError(t, err)

	go func() {
		for i := 0; i < num_messages; i++ {
			source1 <- 1
		}
		close(source1)
	}()

	go func() {
		for i := 0; i < num_messages; i++ {
			source2 <- 2
		}
		close(source2)
	}()

	go func() {
		for i := 0; i < num_messages; i++ {
			source3 <- 3
		}
		close(source3)
	}()

	scheduler.Start()

	s1Count := 0
	s2Count := 0
	s3Count := 0

	for v := range sink {
		switch v {
		case 1:
			s1Count++
		case 2:
			s2Count++
		case 3:
			s3Count++
		}
		test.LessEq(t, 2, AbsDiff(s1Count, s2Count))
		test.LessEq(t, 2, AbsDiff(s1Count, s3Count))
		test.LessEq(t, 2, AbsDiff(s2Count, s3Count))
		// TODO: Replace with using synctest. See #34
		time.Sleep(5 * time.Millisecond) // Give all sources a chance to send
	}

	test.Eq(t, num_messages, s1Count)
	test.Eq(t, num_messages, s2Count)
	test.Eq(t, num_messages, s3Count)

	scheduler.Stop()
}

func TestStartClosingSource(t *testing.T) {
	sink := make(chan int)
	scheduler := NewFairScheduler(sink)
	test.NotNil(t, scheduler)

	source1 := make(chan int)
	err := scheduler.AddSource(source1)
	must.NoError(t, err)

	source2 := make(chan int)
	err = scheduler.AddSource(source2)
	must.NoError(t, err)

	scheduler.Start()

	go func() {
		source1 <- 42
		source2 <- 43
		source1 <- 44
		close(source1)
		source2 <- 45
		source2 <- 46
		close(source2)
	}()

	test.Eq(t, 42, <-sink)
	test.Eq(t, 43, <-sink)
	test.Eq(t, 44, <-sink)
	test.Eq(t, 45, <-sink)
	test.Eq(t, 46, <-sink)

	scheduler.Stop()
}

func TestStop(t *testing.T) {
	sink := make(chan int)
	scheduler := NewFairScheduler(sink)
	test.NotNil(t, scheduler)

	source1 := make(chan int)
	err := scheduler.AddSource(source1)
	must.NoError(t, err)

	go func() {
		for {
			source1 <- 42
		}
	}()

	scheduler.Start()

	test.Eq(t, 42, <-sink)
	test.Eq(t, 42, <-sink)
	test.Eq(t, 42, <-sink)

	time.Sleep(10 * time.Millisecond) // Give the scheduler a chance to settle

	scheduler.Stop()

	time.Sleep(10 * time.Millisecond) // Give the scheduler a chance to settle

	test.Eq(t, 42, <-sink) // Last element in the sink

	_, ok := <-sink
	test.False(t, ok)
}

func TestStopBeforeStart(t *testing.T) {
	sink := make(chan int)
	scheduler := NewFairScheduler(sink)
	test.NotNil(t, scheduler)

	fairScheduler, ok := scheduler.(*fairScheduler[int])
	if !ok {
		t.Error("Returned scheduler is not a fair scheduler")
	}

	source1 := make(chan int)
	err := scheduler.AddSource(source1)
	must.NoError(t, err)

	scheduler.Stop()

	test.Nil(t, fairScheduler.stop)
}
