package ringbuffer

import (
	"sync"

	boxenutil "github.com/carlmontanari/boxen/util"
)

// RingBuffer is a circular buffer for bytes.
type RingBuffer struct {
	lock     *sync.Mutex
	Size     uint32
	ReadPos  uint32
	WritePos uint32
	Content  []byte
}

// NewRingBuffer instantiates a new RingBuffer of size.
func NewRingBuffer(size uint32) *RingBuffer {
	return &RingBuffer{
		lock:     &sync.Mutex{},
		Size:     size,
		ReadPos:  0,
		WritePos: 0,
		Content:  make([]byte, size),
	}
}

// Write puts the contents of b into the ring buffer, overflowing and wrapping around the buffer as
// needed.
func (rb *RingBuffer) Write(b []byte) (int, error) {
	rb.lock.Lock()
	defer rb.lock.Unlock()

	lb := boxenutil.MustIntToUint32(len(b))

	if lb == 0 {
		return 0, nil
	}

	if lb > rb.Size {
		// greater than the size of our ring, we'll replace the whole buffer and reset the read
		// and write pointers. note also that we take the *trailing part* of input b because in
		// our case we care only about the latest data basically
		copy(rb.Content, b[lb-rb.Size:(lb-rb.Size)+rb.Size])

		rb.WritePos = 0
		rb.ReadPos = 0

		return int(lb), nil
	}

	remainingPositions := rb.Size - rb.WritePos

	if remainingPositions <= lb {
		// this is the "wrap around" case -- here we need to put the start of the new buf at the
		// end of our buffer, then overflow all the remaining content to the beginning. when doing
		// this we also need to reposition the read pointer since the slice has been reconfigured
		// and the old read position is now incorrect
		tailChunk := b[:remainingPositions]
		headChunk := b[remainingPositions:]

		rb.Content = append(rb.Content[:rb.WritePos], tailChunk...)
		rb.WritePos = uint32(len(headChunk)) //nolint:gosec
		rb.Content = append(headChunk, rb.Content[rb.WritePos:]...)

		rb.updateReadPos(lb)

		return int(lb), nil
	}

	// finally this is the simplest case where we insert the new buffer into the ring starting at
	// the latest write pointer. we then increment the write and read pointers accordingly.

	rb.Content = append(
		rb.Content[:rb.WritePos],
		append(b, rb.Content[rb.WritePos+lb:rb.Size]...)...)

	rb.WritePos += lb

	rb.updateReadPos(lb)

	return int(lb), nil
}

// Read fills b with content from the buffer. If b is larger than the buffer Read stops filling
// after the buffer has been consumed. The content of b may be filled with 0s/nulls if the
// RingBuffer was just initialized and has not been filled yet.
func (rb *RingBuffer) Read(b []byte) (int, error) {
	rb.lock.Lock()
	defer rb.lock.Unlock()

	lb := boxenutil.MustIntToUint32(len(b))

	if lb == 0 {
		return 0, nil
	}

	if lb > rb.Size {
		// if the buf is larger than our ring buffer, then we copy from our current read pos to the
		// end of the buffer, then *also* the front of the buffer to the read position. in this case
		// the read position can be left alone since we have gone fully around the circle and are
		// back to where we started
		n := copy(b, rb.Content[rb.ReadPos:])
		n += copy(b[n:], rb.Content[:rb.ReadPos])

		return n, nil
	}

	remainingPositions := rb.Size - rb.ReadPos

	if remainingPositions <= lb {
		n := copy(b, rb.Content[rb.ReadPos:])
		n += copy(b[n:], rb.Content[:lb-uint32(n)]) //nolint:gosec

		rb.updateReadPos(lb)

		return n, nil
	}

	// finally the simplest case again -- here we just copy from current read position to len of
	// b and then update the read pointer when we're done

	n := copy(b, rb.Content[rb.ReadPos:rb.ReadPos+lb])

	rb.updateReadPos(lb)

	return n, nil
}

// GetContent safely (via lock) returns the contents of the buffer.
func (rb *RingBuffer) GetContent() []byte {
	rb.lock.Lock()
	defer rb.lock.Unlock()

	out := make([]byte, len(rb.Content))
	copy(out, rb.Content)

	return out
}

// GetOrderedContent safely (via lock) returns the contents of the buffer. It does this from the
// current write position backwards basically -- meaning it shows the contents of the "circular"
// buffer in the order it was received rather than a wrapped circle.
func (rb *RingBuffer) GetOrderedContent() []byte {
	rb.lock.Lock()
	defer rb.lock.Unlock()

	out := make([]byte, len(rb.Content))

	// take write pos +1 and go to end of buffer
	// then take zero and go to write position
	n := copy(out, rb.Content[rb.WritePos:rb.Size-1])
	copy(out[n:], rb.Content[:rb.WritePos])

	return out
}

// Reset purges or resets the ringbuffer deleting all data and zeroizing the read/write pointers.
func (rb *RingBuffer) Reset() {
	rb.lock.Lock()
	defer rb.lock.Unlock()

	rb.Content = make([]byte, rb.Size)
	rb.ReadPos = 0
	rb.WritePos = 0
}

// SetReadPos updates the read position pointer to the given value, it fatals if the given position
// is not <= rb.Size.
func (rb *RingBuffer) SetReadPos(pos uint32) {
	rb.lock.Lock()
	defer rb.lock.Unlock()

	if pos > rb.Size {
		panic("invalid read position requested")
	}

	rb.ReadPos = pos
}

func (rb *RingBuffer) updateReadPos(offset uint32) {
	newReadPos := rb.ReadPos + offset
	if newReadPos >= rb.Size {
		newReadPos -= rb.Size
	}

	rb.ReadPos = newReadPos
}
