// Package nav provides a navigation stack for screens in the TUI.
package nav

// Stack is a LIFO stack used to push/pop screens. Zero value is empty.
type Stack[T any] struct {
	items []T
}

// Push adds t to the top of the stack.
func (s *Stack[T]) Push(t T) { s.items = append(s.items, t) }

// Pop removes and returns the top, or zero+false when empty.
// The slot is cleared so popped items don't leak.
func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	if len(s.items) == 0 {
		return zero, false
	}
	top := s.items[len(s.items)-1]
	s.items[len(s.items)-1] = zero
	s.items = s.items[:len(s.items)-1]
	return top, true
}

// Top returns the top element without removing it, or zero+false when empty.
func (s *Stack[T]) Top() (T, bool) {
	var zero T
	if len(s.items) == 0 {
		return zero, false
	}
	return s.items[len(s.items)-1], true
}

// Replace swaps the top element. No-op when empty.
func (s *Stack[T]) Replace(t T) {
	if len(s.items) == 0 {
		s.items = append(s.items, t)
		return
	}
	s.items[len(s.items)-1] = t
}

// Len returns the number of items.
func (s *Stack[T]) Len() int { return len(s.items) }
