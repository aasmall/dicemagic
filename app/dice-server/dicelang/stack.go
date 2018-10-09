package dicelang

import (
	"bytes"
	"fmt"
)

type Stack struct {
	top  *Element
	size int
}

type Element struct {
	value interface{}
	next  *Element
}

func (s *Stack) Empty() bool {
	return s.size == 0
}

func (s *Stack) Top() interface{} {
	return s.top.value
}

func (s *Stack) Push(value interface{}) {
	s.top = &Element{value, s.top}
	s.size++
}

func (s *Stack) Pop() (value interface{}) {
	if s.size > 0 {
		value, s.top = s.top.value, s.top.next
		s.size--
		return
	}
	return nil
}

func (s *Stack) String() string {
	var buf bytes.Buffer
	var next *Element
	if s.Empty() {
		return ""
	}
	buf.WriteString(fmt.Sprintf("Top: %+v\n", s.top.value))
	next = s.top.next
	for next != nil {
		buf.WriteString(fmt.Sprintf("     %+v\n", next.value))
		next = next.next
	}
	return buf.String()
}

// func (s *Stack) String() string {
// 	ch := make(chan interface{})
// 	go func() {
// 		emitChildren(ch, s.top)
// 		close(ch)
// 	}()
// 	var buf bytes.Buffer
// 	for element := range ch {
// 		buf.WriteString(fmt.Sprintf("%v\n", element))
// 	}
// 	return buf.String()
// }

// func emitChildren(ch chan interface{}, e *Element) {

// 	if e != nil {
// 		ch <- e.value
// 	}
// 	if e.next != nil {
// 		emitChildren(ch, e.next)
// 	}
// }
