package dicelang

import (
	"bytes"
	"fmt"
)

// Stack replicates the behavior of a push-down-stack, but it's really just a linked list.
type Stack struct {
	top  *Element
	size int
}

// Element contains the value of an element and a pointer to the next element in the stack.
type Element struct {
	value interface{}
	next  *Element
}

// Empty returns true if the stack is empty
func (s *Stack) Empty() bool {
	return s.size == 0
}

// Top returns the element at the top of the stack
func (s *Stack) Top() interface{} {
	if s.Empty() {
		return nil
	}
	return s.top.value
}

// Push adds an element to the top of the stack
func (s *Stack) Push(value interface{}) {
	s.top = &Element{value, s.top}
	s.size++
}

// Pop removes an element from the top of the stack and returns it
func (s *Stack) Pop() (value interface{}) {
	if s.size > 0 {
		value, s.top = s.top.value, s.top.next
		s.size--
		return
	}
	return nil
}

// String pretty prints the whole stack
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
