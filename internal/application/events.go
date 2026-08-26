package application

type EventBus struct{ events []string }

func (b *EventBus) Publish(name string) { b.events = append(b.events, name) }
func (b *EventBus) Names() []string     { return append([]string{}, b.events...) }
