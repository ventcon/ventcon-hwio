package scheduling

type Scheduler[T any] interface {
	AddSource(<-chan T) error
	Start()
	Stop()
}
