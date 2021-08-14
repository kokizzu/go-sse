package event

import "io"

// Name is the event field that sets the event's type.
type Name string

func (n Name) name() string {
	return "event"
}

func (n Name) apply(e *Event) {
	if e.nameIndex == -1 {
		e.nameIndex = len(e.fields)
		e.fields = append(e.fields, n)
	} else {
		e.fields[e.nameIndex] = n
	}
}

func (n Name) Message(w io.Writer) error {
	_, err := w.Write([]byte(n))

	return err
}