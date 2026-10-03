package agent

import (
	"errors"
	"strings"
	"testing"
)

func TestDrainReads(t *testing.T) {
	chunks := []string{"a", "b", "c", ""}

	read := func() ([]byte, error) {
		chunk := chunks[0]
		chunks = chunks[1:]

		return []byte(chunk), nil
	}

	got, err := drainReads(read)
	if err != nil || string(got) != "abc" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestDrainReadsBounded(t *testing.T) {
	calls := 0

	got, err := drainReads(func() ([]byte, error) {
		calls++

		return []byte("x"), nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if calls != maxDrainReads || len(got) != maxDrainReads {
		t.Fatalf(
			"expected %d bounded reads, got %d calls and %d bytes",
			maxDrainReads,
			calls,
			len(got),
		)
	}
}

var errTestRead = errors.New("read failed")

func TestDrainReadsError(t *testing.T) {
	errRead := errTestRead
	calls := 0

	got, err := drainReads(func() ([]byte, error) {
		calls++
		if calls == 2 {
			return nil, errRead
		}

		return []byte(strings.Repeat("y", 3)), nil
	})
	if !errors.Is(err, errRead) || string(got) != "yyy" {
		t.Fatalf("expected the data read before the error and the error, got %q, %v", got, err)
	}
}
