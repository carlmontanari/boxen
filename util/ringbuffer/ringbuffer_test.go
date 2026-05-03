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

func TestRingBufferGetContentProperlySized(t *testing.T) {
	rb := boxenutilringbuffer.NewRingBuffer(100)

	assertInitialState(t, rb, 100)

	content := []byte{1, 2, 3}

	n, err := rb.Write(content)
	if err != nil {
		t.Fatalf("write caused error %v", err)
	}

	if n != len(content) {
		t.Fatalf("write length incorrect, got %d, want %d", n, len(content))
	}

	getContentLen := len(rb.GetContent())
	expectedContentLen := len(content)

	if getContentLen != expectedContentLen {
		t.Fatalf(
			"returned buffer size, not content size, got %d, want %d",
			getContentLen,
			expectedContentLen,
		)
	}
}

func TestRingBufferGetOrderedContent(t *testing.T) {
	rb := boxenutilringbuffer.NewRingBuffer(5)

	_, err := rb.Write([]byte{1, 2, 3, 4, 5})
	if err != nil {
		t.Fatalf("errored writing content, error: %v", err.Error())
	}

	expected := []byte{1, 2, 3, 4, 5}

	actual := rb.GetOrderedContent()
	if !bytes.Equal(actual, expected) {
		t.Fatalf(
			"ordered content incorrect, got %d, want %d",
			actual,
			expected,
		)
	}

	_, err = rb.Write([]byte{6})
	if err != nil {
		t.Fatalf("errored writing content, error: %v", err.Error())
	}

	expected = []byte{2, 3, 4, 5, 6}

	actual = rb.GetOrderedContent()
	if !bytes.Equal(actual, []byte{2, 3, 4, 5, 6}) {
		t.Fatalf(
			"ordered content incorrect, got %d, want %d",
			actual,
			expected,
		)
	}

	_, err = rb.Write([]byte{7, 8, 9, 10, 11})
	if err != nil {
		t.Fatalf("errored writing content, error: %v", err.Error())
	}

	expected = []byte{7, 8, 9, 10, 11}

	actual = rb.GetOrderedContent()
	if !bytes.Equal(actual, expected) {
		t.Fatalf(
			"ordered content incorrect, got %d, want %d",
			actual,
			expected,
		)
	}
}
