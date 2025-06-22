package stack

import (
	"testing"
)

func TestStack_New(t *testing.T) {
	s := New[int]()

	if !s.IsEmpty() {
		t.Error("New() stack should be empty")
	}

	if s.Size() != 0 {
		t.Errorf("New() stack size = %d, want 0", s.Size())
	}
}

func TestStack_NewWithCapacity(t *testing.T) {
	s := NewWithCapacity[string](10)

	if !s.IsEmpty() {
		t.Error("NewWithCapacity() stack should be empty")
	}

	if s.Size() != 0 {
		t.Errorf("NewWithCapacity() stack size = %d, want 0", s.Size())
	}
}

func TestStack_PushAndPop(t *testing.T) {
	s := New[int]()

	s.Push(1)
	s.Push(2)
	s.Push(3)

	if s.Size() != 3 {
		t.Errorf("Push() stack size = %d, want 3", s.Size())
	}

	if s.IsEmpty() {
		t.Error("Push() stack should not be empty")
	}

	// LIFO order
	val, ok := s.Pop()
	if !ok || val != 3 {
		t.Errorf("Pop() = %d, %t, want 3, true", val, ok)
	}

	val, ok = s.Pop()
	if !ok || val != 2 {
		t.Errorf("Pop() = %d, %t, want 2, true", val, ok)
	}

	val, ok = s.Pop()
	if !ok || val != 1 {
		t.Errorf("Pop() = %d, %t, want 1, true", val, ok)
	}

	val, ok = s.Pop()
	if ok || val != 0 {
		t.Errorf("Pop() from empty stack = %d, %t, want 0, false", val, ok)
	}

	if !s.IsEmpty() {
		t.Error("Pop() stack should be empty after popping all elements")
	}
}

func TestStack_Peek(t *testing.T) {
	s := New[string]()

	val, ok := s.Peek()
	if ok || val != "" {
		t.Errorf("Peek() on empty stack = %q, %t, want \"\", false", val, ok)
	}

	s.Push("first")
	s.Push("second")

	val, ok = s.Peek()
	if !ok || val != "second" {
		t.Errorf("Peek() = %q, %t, want \"second\", true", val, ok)
	}

	// Ensure peek doesn't modify stack
	if s.Size() != 2 {
		t.Errorf("Peek() changed stack size to %d, want 2", s.Size())
	}

	val, ok = s.Peek()
	if !ok || val != "second" {
		t.Errorf("Second Peek() = %q, %t, want \"second\", true", val, ok)
	}
}

func TestStack_PeekRef(t *testing.T) {
	s := New[int]()

	ref := s.PeekRef()
	if ref != nil {
		t.Error("PeekRef() on empty stack should return nil")
	}

	s.Push(42)
	s.Push(100)

	ref = s.PeekRef()
	if ref == nil {
		t.Fatal("PeekRef() should not return nil for non-empty stack")
	}

	if *ref != 100 {
		t.Errorf("PeekRef() = %d, want 100", *ref)
	}

	// Test modifying through reference
	*ref = 200

	val, _ := s.Peek()
	if val != 200 {
		t.Errorf("After modifying through PeekRef(), top element = %d, want 200", val)
	}
}

func TestStack_ToSlice(t *testing.T) {
	s := New[int]()
	s.Push(1)
	s.Push(2)
	s.Push(3)

	slice := s.ToSlice()

	expected := []int{1, 2, 3}
	if len(slice) != len(expected) {
		t.Errorf("ToSlice() length = %d, want %d", len(slice), len(expected))
	}

	for i, val := range expected {
		if slice[i] != val {
			t.Errorf("ToSlice()[%d] = %d, want %d", i, slice[i], val)
		}
	}

	// Ensure modifying slice doesn't affect stack
	slice[0] = 999

	bottomSlice := s.ToSlice()
	if bottomSlice[0] != 1 {
		t.Errorf("After modifying ToSlice() result, original stack changed: got %d, want 1", bottomSlice[0])
	}
}

func TestStack_Contains(t *testing.T) {
	s := New[string]()
	s.Push("apple", "banana", "cherry")

	if !Contains(s, "banana") {
		t.Error("Contains(\"banana\") should return true")
	}

	if !Contains(s, "apple") {
		t.Error("Contains(\"apple\") should return true")
	}

	if Contains(s, "orange") {
		t.Error("Contains(\"orange\") should return false")
	}
}

func TestStack_Reverse(t *testing.T) {
	s := New[int]()
	s.Push(1, 2, 3, 4, 5)

	s.Reverse()

	// After reverse, order should be: bottom->top becomes 5,4,3,2,1
	expected := []int{5, 4, 3, 2, 1}
	slice := s.ToSlice()

	for i, val := range expected {
		if slice[i] != val {
			t.Errorf("After Reverse(), ToSlice()[%d] = %d, want %d", i, slice[i], val)
		}
	}

	val, _ := s.Pop()
	if val != 1 {
		t.Errorf("After Reverse(), Pop() = %d, want 1", val)
	}
}

func TestStack_Clone(t *testing.T) {
	s := New[int]()
	s.Push(1, 2, 3, 4, 5)

	cloned := s.Clone()

	if cloned.Size() != s.Size() {
		t.Errorf("Clone() size = %d, want %d", cloned.Size(), s.Size())
	}

	originalSlice := s.ToSlice()
	clonedSlice := cloned.ToSlice()

	for i, val := range originalSlice {
		if clonedSlice[i] != val {
			t.Errorf("Clone()[%d] = %d, want %d", i, clonedSlice[i], val)
		}
	}

	// Test that modifying clone doesn't affect original
	cloned.Push(999)

	if s.Size() == cloned.Size() {
		t.Error("Modifying clone affected original stack")
	}

	val, _ := s.Peek()
	if val == 999 {
		t.Error("Modifying clone affected original stack contents")
	}

	empty := New[int]()
	emptyClone := empty.Clone()

	if !emptyClone.IsEmpty() {
		t.Error("Clone() of empty stack should be empty")
	}
}

func TestStack_EmptyStack(t *testing.T) {
	s := New[int]()

	if !s.IsEmpty() {
		t.Error("Empty stack IsEmpty() should return true")
	}

	if s.Size() != 0 {
		t.Errorf("Empty stack Size() = %d, want 0", s.Size())
	}

	slice := s.ToSlice()
	if len(slice) != 0 {
		t.Errorf("Empty stack ToSlice() length = %d, want 0", len(slice))
	}
}

func TestStack_GenericTypes(t *testing.T) {
	type TestStruct struct {
		Name string
		ID   int
	}

	s := New[TestStruct]()
	s.Push(TestStruct{Name: "first", ID: 1})
	s.Push(TestStruct{Name: "second", ID: 2})

	val, ok := s.Pop()
	if !ok || val.Name != "second" || val.ID != 2 {
		t.Errorf("Pop() = %+v, %t, want {Name:second ID:2}, true", val, ok)
	}

	ps := New[*TestStruct]()
	obj1 := &TestStruct{Name: "obj1", ID: 1}
	obj2 := &TestStruct{Name: "obj2", ID: 2}

	ps.Push(obj1)
	ps.Push(obj2)

	pval, ok := ps.Pop()
	if !ok || pval != obj2 {
		t.Errorf("Pop() = %p, %t, want %p, true", pval, ok, obj2)
	}
}
