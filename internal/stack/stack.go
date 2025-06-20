package stack

import (
	"slices"
)

// Stack represents a generic stack data structure.
// It provides LIFO (Last In, First Out) semantics with type safety.
type Stack[T any] struct {
	items []T
}

// New creates a new empty stack.
func New[T any]() *Stack[T] {
	return &Stack[T]{}
}

// NewWithCapacity creates a new stack with the specified initial capacity.
// This can help reduce allocations when the approximate stack size is known.
func NewWithCapacity[T any](capacity int) *Stack[T] {
	return &Stack[T]{
		items: make([]T, 0, capacity),
	}
}

// Push adds multiple elements to the top of the stack.
// Elements are pushed in order, so the last element will be at the top.
func (s *Stack[T]) Push(items ...T) {
	s.items = append(s.items, items...)
}

// Pop removes and returns the top element from the stack.
// Returns the zero value and false if the stack is empty.
func (s *Stack[T]) Pop() (T, bool) {
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}

	index := len(s.items) - 1
	item := s.items[index]
	s.items = s.items[:index]
	return item, true
}

// Peek returns the top element without removing it.
// Returns the zero value and false if the stack is empty.
func (s *Stack[T]) Peek() (T, bool) {
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}

	return s.items[len(s.items)-1], true
}

// PeekRef returns a pointer to the top element without removing it.
// Returns nil if the stack is empty.
// This is useful when you need to modify the top element in place.
func (s *Stack[T]) PeekRef() *T {
	if len(s.items) == 0 {
		return nil
	}

	return &s.items[len(s.items)-1]
}

// IsEmpty returns true if the stack contains no elements.
func (s *Stack[T]) IsEmpty() bool {
	return len(s.items) == 0
}

// Size returns the number of elements in the stack.
func (s *Stack[T]) Size() int {
	return len(s.items)
}

// ToSlice returns a copy of the stack contents as a slice.
// The slice is ordered from bottom to top of the stack.
func (s *Stack[T]) ToSlice() []T {
	return slices.Clone(s.items)
}

// Contains returns true if the stack contains the specified element.
// Uses == for comparison, so it only works with comparable types.
// This method is only available when T is comparable.
func Contains[T comparable](s *Stack[T], item T) bool {
	return slices.Contains(s.items, item)
}

// Reverse reverses the order of elements in the stack.
// The bottom becomes the top and vice versa.
func (s *Stack[T]) Reverse() {
	slices.Reverse(s.items)
}

// Clone creates a deep copy of the stack.
func (s *Stack[T]) Clone() *Stack[T] {
	return &Stack[T]{
		items: slices.Clone(s.items),
	}
}
