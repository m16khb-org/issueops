package selfverify

import "bytes"

// CommandOutput retains the raw suffix of a stream while counting all bytes.
// Each stream has its own writer; Result is called after writing has finished.
type CommandOutput struct {
	budget int
	total  int
	ring   []byte
	next   int
	full   bytes.Buffer
}

func NewCommandOutput(budget int) *CommandOutput {
	return &CommandOutput{budget: budget}
}

func (o *CommandOutput) Write(p []byte) (int, error) {
	n := len(p)
	o.total += n
	if o.budget <= 0 {
		return o.full.Write(p)
	}
	retained := min(o.total, o.budget)
	if retained > cap(o.ring) {
		ring := make([]byte, retained, min(o.budget, max(retained, 2*cap(o.ring))))
		copy(ring, o.ring)
		o.ring = ring
	} else {
		o.ring = o.ring[:retained]
	}
	if n >= o.budget {
		copy(o.ring, p[n-o.budget:])
		o.next = 0
	} else {
		first := copy(o.ring[o.next:], p)
		copy(o.ring, p[first:])
		o.next = (o.next + n) % o.budget
	}
	return n, nil
}

func (o *CommandOutput) Result() (string, bool, int) {
	if o.budget <= 0 {
		return o.full.String(), false, o.total
	}
	retained := min(o.total, o.budget)
	tail := make([]byte, retained)
	start := 0
	if o.total >= o.budget {
		start = o.next
	}
	first := copy(tail, o.ring[start:start+min(retained, o.budget-start)])
	copy(tail[first:], o.ring[:retained-first])
	return tailWithBudget(string(tail), o.total, o.budget)
}
