package cache

import (
	"testing"
	"time"
)

func TestTTLCache_SetAndGet(t *testing.T) {
	c := New[string](time.Minute)

	if _, ok := c.Get("missing"); ok {
		t.Fatal("expected miss for a key that was never set")
	}

	c.Set("k1", "v1")
	got, ok := c.Get("k1")
	if !ok || got != "v1" {
		t.Fatalf("Get(k1) = (%q, %v), want (%q, true)", got, ok, "v1")
	}
}

func TestTTLCache_Expires(t *testing.T) {
	c := New[string](time.Minute)

	current := time.Now()
	c.now = func() time.Time { return current }

	c.Set("k1", "v1")

	current = current.Add(59 * time.Second)
	if _, ok := c.Get("k1"); !ok {
		t.Fatal("expected hit just before TTL elapses")
	}

	current = current.Add(2 * time.Second)
	if _, ok := c.Get("k1"); ok {
		t.Fatal("expected miss after TTL elapses")
	}
}
