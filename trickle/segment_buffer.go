package trickle

import (
	"sync/atomic"
)

const (
	segmentBufferInitialPageSize = 32 * 1024
	segmentBufferMaxPageSize     = 1024 * 1024
)

type segmentPage struct {
	start   int
	data    []byte
	written int
}

type segmentPageList struct {
	pages []*segmentPage
}

// segmentBuffer is an append-only paged buffer tailored for Segment fanout.
// Pages are immutable containers whose published prefix is tracked atomically,
// allowing readers to take a lock-free fast path when data is already available.
type segmentBuffer struct {
	pageList     atomic.Pointer[segmentPageList]
	published    atomic.Int64
	totalWritten int
	initialCap   int
	maxPageCap   int
	nextPageCap  int
}

func newSegmentBuffer() *segmentBuffer {
	return newSegmentBufferWithPageCaps(segmentBufferInitialPageSize, segmentBufferMaxPageSize)
}

func newSegmentBufferWithPageCaps(initialCap, maxPageCap int) *segmentBuffer {
	if initialCap <= 0 {
		initialCap = segmentBufferInitialPageSize
	}
	if maxPageCap < initialCap {
		maxPageCap = initialCap
	}
	b := &segmentBuffer{
		initialCap:  initialCap,
		maxPageCap:  maxPageCap,
		nextPageCap: initialCap,
	}
	b.pageList.Store(&segmentPageList{pages: nil})
	return b
}

func (b *segmentBuffer) write(data []byte) {
	for len(data) > 0 {
		page := b.ensureTailPage()
		written := page.written
		remaining := cap(page.data) - written
		if remaining > len(data) {
			remaining = len(data)
		}

		end := written + remaining
		copy(page.data[written:end], data[:remaining])
		page.written = end

		data = data[remaining:]
		b.totalWritten += remaining
	}
	b.published.Store(int64(b.totalWritten))
}

func (b *segmentBuffer) ensureTailPage() *segmentPage {
	list := b.pageList.Load()
	if list != nil && len(list.pages) > 0 {
		tail := list.pages[len(list.pages)-1]
		if tail.written < cap(tail.data) {
			return tail
		}
	}

	pageCap := b.nextPageCap
	if pageCap == 0 {
		pageCap = b.initialCap
	}
	page := &segmentPage{
		start: b.totalWritten,
		data:  make([]byte, pageCap),
	}
	newPages := make([]*segmentPage, 0, len(list.pages)+1)
	newPages = append(newPages, list.pages...)
	newPages = append(newPages, page)
	b.pageList.Store(&segmentPageList{pages: newPages})

	if b.nextPageCap < b.maxPageCap {
		b.nextPageCap *= 2
		if b.nextPageCap > b.maxPageCap {
			b.nextPageCap = b.maxPageCap
		}
	}
	return page
}

func (b *segmentBuffer) isEmpty() bool {
	return b.published.Load() == 0
}

func (b *segmentBuffer) readChunk(pos int) ([]byte, int, bool, bool, int) {
	if pos < 0 {
		return nil, pos, false, true, int(b.published.Load())
	}
	published := int(b.published.Load())
	if pos > published {
		return nil, pos, false, true, published
	}
	if pos == published {
		return nil, pos, false, false, published
	}

	list := b.pageList.Load()
	if list == nil {
		return nil, pos, false, false, published
	}

	for i, page := range list.pages {
		pageEnd := published
		if i+1 < len(list.pages) {
			pageEnd = list.pages[i+1].start
			if pageEnd > published {
				pageEnd = published
			}
		}
		if pos < page.start || pos >= pageEnd {
			continue
		}

		offset := pos - page.start
		data := page.data[offset : pageEnd-page.start]
		return data, pageEnd, pageEnd == published, false, published
	}

	// We may have observed a newer published cursor than the currently visible
	// page-list snapshot. Treat this as "not available yet" so callers can retry
	// under synchronization instead of incorrectly signaling EOF.
	return nil, pos, false, false, published
}
