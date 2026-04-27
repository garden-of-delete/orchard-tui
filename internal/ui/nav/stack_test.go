package nav

import "testing"

func TestStack(t *testing.T) {
	var s Stack[string]
	if s.Len() != 0 {
		t.Fatalf("len = %d", s.Len())
	}
	if _, ok := s.Top(); ok {
		t.Fatal("Top on empty returned ok")
	}
	if _, ok := s.Pop(); ok {
		t.Fatal("Pop on empty returned ok")
	}
	s.Push("a")
	s.Push("b")
	if got, _ := s.Top(); got != "b" {
		t.Errorf("Top = %q", got)
	}
	if got, _ := s.Pop(); got != "b" {
		t.Errorf("Pop = %q", got)
	}
	s.Replace("c")
	if got, _ := s.Top(); got != "c" {
		t.Errorf("after Replace Top = %q", got)
	}
	if s.Len() != 1 {
		t.Errorf("len = %d", s.Len())
	}
	if _, ok := s.Pop(); !ok {
		t.Error("Pop after Replace failed")
	}
}
