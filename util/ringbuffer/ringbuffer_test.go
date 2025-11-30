package ringbuffer_test

import (
	"bytes"
	"testing"

	boxenutilringbuffer "github.com/carlmontanari/boxen/util/ringbuffer"
)

func assertInitialState(t *testing.T, rb *boxenutilringbuffer.RingBuffer, size uint32) {
	t.Helper()

	if rb.Size != size {
		t.Fatalf(
			"initial buffer size incorrect, got %d, want %d",
			rb.Size,
			size,
		)
	}

	if rb.ReadPos != 0 {
		t.Fatalf(
			"initial read position incorrect, got %d, want %d",
			rb.ReadPos,
			0,
		)
	}

	if rb.WritePos != 0 {
		t.Fatalf(
			"initial write position incorrect, got %d, want %d",
			rb.WritePos,
			0,
		)
	}
}

type testWrite struct {
	buf              []byte
	expectedContent  []byte
	expectedReadPos  uint32
	expectedWritePos uint32
}

func TestRingBufferWrite(t *testing.T) {
	cases := []struct {
		name   string
		size   uint32
		writes []testWrite
	}{
		{
			name: "simple",
			size: 3,
			writes: []testWrite{
				{
					buf:              []byte{1},
					expectedContent:  []byte{1, 0, 0},
					expectedReadPos:  1,
					expectedWritePos: 1,
				},
			},
		},
		{
			name: "full",
			size: 3,
			writes: []testWrite{
				{
					buf:              []byte{1},
					expectedContent:  []byte{1, 0, 0},
					expectedReadPos:  1,
					expectedWritePos: 1,
				},
				{
					buf:             []byte{2, 3},
					expectedContent: []byte{1, 2, 3},
					// read/write wrap around to start
					expectedReadPos:  0,
					expectedWritePos: 0,
				},
			},
		},
		{
			name: "overflow",
			size: 3,
			writes: []testWrite{
				{
					buf:              []byte{1},
					expectedContent:  []byte{1, 0, 0},
					expectedReadPos:  1,
					expectedWritePos: 1,
				},
				{
					buf:             []byte{2, 3, 4},
					expectedContent: []byte{4, 2, 3},
					// read/write wrap around past the start
					expectedReadPos:  1,
					expectedWritePos: 1,
				},
			},
		},
		{
			name: "oversized-write",
			size: 3,
			writes: []testWrite{
				{
					buf:              []byte{1, 2, 3, 4, 5},
					expectedContent:  []byte{3, 4, 5},
					expectedReadPos:  0,
					expectedWritePos: 0,
				},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				rb := boxenutilringbuffer.NewRingBuffer(testCase.size)

				assertInitialState(t, rb, testCase.size)

				for _, write := range testCase.writes {
					n, err := rb.Write(write.buf)
					if err != nil {
						t.Fatalf("write caused error %v", err)
					}

					if n != len(write.buf) {
						t.Fatalf("write length incorrect, got %d, want %d", n, len(write.buf))
					}

					if !bytes.Equal(rb.Content, write.expectedContent) {
						t.Fatalf(
							"buffer content incorrect, got %v, want %v",
							rb.Content,
							write.expectedContent,
						)
					}

					if rb.ReadPos != write.expectedReadPos {
						t.Fatalf(
							"read position incorrect, got %d, want %d",
							rb.ReadPos,
							write.expectedReadPos,
						)
					}

					if rb.WritePos != write.expectedWritePos {
						t.Fatalf(
							"write position incorrect, got %d, want %d",
							rb.WritePos,
							write.expectedWritePos,
						)
					}
				}
			},
		)
	}
}

type testRead struct {
	buf              []byte
	expectedContent  []byte
	expectedReadPos  uint32
	expectedWritePos uint32
}

func TestRingBufferRead(t *testing.T) {
	cases := []struct {
		name    string
		size    uint32
		content []byte
		reads   []testRead
	}{
		{
			name:    "simple",
			size:    3,
			content: []byte{1, 2, 3},
			reads: []testRead{
				{
					buf:              []byte{0},
					expectedContent:  []byte{1},
					expectedReadPos:  1,
					expectedWritePos: 0,
				},
			},
		},
		{
			name:    "full",
			size:    3,
			content: []byte{1, 2, 3},
			reads: []testRead{
				{
					buf:              []byte{0, 0, 0},
					expectedContent:  []byte{1, 2, 3},
					expectedReadPos:  0,
					expectedWritePos: 0,
				},
			},
		},
		{
			name:    "subsequent-read",
			size:    3,
			content: []byte{1, 2, 3},
			reads: []testRead{
				{
					buf:              []byte{0, 0, 0},
					expectedContent:  []byte{1, 2, 3},
					expectedReadPos:  0,
					expectedWritePos: 0,
				},
				{
					buf:              []byte{0, 0},
					expectedContent:  []byte{1, 2},
					expectedReadPos:  2,
					expectedWritePos: 0,
				},
			},
		},
		{
			name:    "overflow-read",
			size:    3,
			content: []byte{1, 2, 3},
			reads: []testRead{
				{
					buf:              []byte{0, 0, 0, 0},
					expectedContent:  []byte{1, 2, 3, 0},
					expectedReadPos:  0,
					expectedWritePos: 0,
				},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				rb := boxenutilringbuffer.NewRingBuffer(testCase.size)

				assertInitialState(t, rb, testCase.size)

				n, err := rb.Write(testCase.content)
				if err != nil {
					t.Fatalf("write caused error %v", err)
				}

				if n != len(testCase.content) {
					t.Fatalf("write length incorrect, got %d, want %d", n, len(testCase.content))
				}

				for _, read := range testCase.reads {
					n, err = rb.Read(read.buf)
					if err != nil {
						t.Fatalf("read caused error %v", err)
					}

					if n != len(read.buf) && n != int(rb.Size) {
						t.Fatalf("read length incorrect, got %d, want %d", n, len(read.buf))
					}

					if !bytes.Equal(read.buf, read.expectedContent) {
						t.Fatalf(
							"read buffer content incorrect, got %v, want %v",
							read.buf,
							read.expectedContent,
						)
					}

					if rb.ReadPos != read.expectedReadPos {
						t.Fatalf(
							"read position incorrect, got %d, want %d",
							rb.ReadPos,
							read.expectedReadPos,
						)
					}

					if rb.WritePos != read.expectedWritePos {
						t.Fatalf(
							"write position incorrect, got %d, want %d",
							rb.WritePos,
							read.expectedWritePos,
						)
					}
				}
			},
		)
	}
}
