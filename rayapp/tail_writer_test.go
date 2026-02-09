package rayapp

import (
	"sync"
	"testing"
)

func TestTailWriter(t *testing.T) {
	t.Run("under limit", func(t *testing.T) {
		tw := newTailWriter(16)
		tw.Write([]byte("hello"))
		if got, want := tw.String(), "hello"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("multiple writes under limit", func(t *testing.T) {
		tw := newTailWriter(16)
		tw.Write([]byte("abc"))
		tw.Write([]byte("def"))
		if got, want := tw.String(), "abcdef"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("exact limit", func(t *testing.T) {
		tw := newTailWriter(5)
		tw.Write([]byte("abcde"))
		if got, want := tw.String(), "abcde"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("wraps around keeps tail", func(t *testing.T) {
		tw := newTailWriter(5)
		tw.Write([]byte("abc"))
		tw.Write([]byte("defgh"))
		if got, want := tw.String(), "defgh"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("multiple wraps", func(t *testing.T) {
		tw := newTailWriter(4)
		tw.Write([]byte("ab"))
		tw.Write([]byte("cd"))
		tw.Write([]byte("ef"))
		if got, want := tw.String(), "cdef"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("single write exceeds limit", func(t *testing.T) {
		tw := newTailWriter(4)
		n, err := tw.Write([]byte("abcdefgh"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 8 {
			t.Errorf("Write() = %d, want 8", n)
		}
		if got, want := tw.String(), "efgh"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("write returns full length", func(t *testing.T) {
		tw := newTailWriter(4)
		tw.Write([]byte("abc"))
		n, err := tw.Write([]byte("defgh"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 5 {
			t.Errorf("Write() = %d, want 5", n)
		}
	})

	t.Run("empty write", func(t *testing.T) {
		tw := newTailWriter(4)
		tw.Write([]byte("ab"))
		tw.Write([]byte(""))
		if got, want := tw.String(), "ab"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("concurrent writes", func(t *testing.T) {
		tw := newTailWriter(256)
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					tw.Write([]byte("abcdefgh"))
				}
			}()
		}
		wg.Wait()

		got := tw.String()
		if len(got) > 256 {
			t.Errorf("String() length = %d, want <= 256", len(got))
		}
	})
}
