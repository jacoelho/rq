package stack

import (
	"slices"
)

type Stack[T any] struct {
	items []T
}

func New[T any]() *Stack[T] {
	return &Stack[T]{}
}

// NewWithCapacity reduces allocations when approximate stack size is known.
func NewWithCapacity[T any](capacity int) *Stack[T] {
	return &Stack[T]{
		items: make([]T, 0, capacity),
	}
}

// Push adds elements in order with the last element at the top.
func (s *Stack[T]) Push(items ...T) {
	s.items = append(s.items, items...)
}

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

func (s *Stack[T]) Peek() (T, bool) {
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}

	return s.items[len(s.items)-1], true
}

// PeekRef allows modifying the top element in place.
func (s *Stack[T]) PeekRef() *T {
	if len(s.items) == 0 {
		return nil
	}

	return &s.items[len(s.items)-1]
}

func (s *Stack[T]) IsEmpty() bool {
	return len(s.items) == 0
}

func (s *Stack[T]) Size() int {
	return len(s.items)
}

// ToSlice orders from bottom to top of the stack.
func (s *Stack[T]) ToSlice() []T {
	return slices.Clone(s.items)
}

// Contains uses == for comparison and only works with comparable types.
func Contains[T comparable](s *Stack[T], item T) bool {
	return slices.Contains(s.items, item)
}

func (s *Stack[T]) Reverse() {
	slices.Reverse(s.items)
}

func (s *Stack[T]) Clone() *Stack[T] {
	return &Stack[T]{
		items: slices.Clone(s.items),
	}
}
