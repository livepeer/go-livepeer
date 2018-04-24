package format

import (
	"context"
	"sync"
	"testing"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

// Test dag
type testDag struct {
	mu    sync.Mutex
	nodes map[string]Node
}

func newTestDag() *testDag {
	return &testDag{nodes: make(map[string]Node)}
}

func (d *testDag) Get(ctx context.Context, cid *cid.Cid) (Node, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if n, ok := d.nodes[cid.KeyString()]; ok {
		return n, nil
	}
	return nil, ErrNotFound
}

func (d *testDag) GetMany(ctx context.Context, cids []*cid.Cid) <-chan *NodeOption {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(chan *NodeOption, len(cids))
	for _, c := range cids {
		if n, ok := d.nodes[c.KeyString()]; ok {
			out <- &NodeOption{Node: n}
		} else {
			out <- &NodeOption{Err: ErrNotFound}
		}
	}
	return out
}

func (d *testDag) Add(ctx context.Context, node Node) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nodes[node.Cid().KeyString()] = node
	return nil
}

func (d *testDag) AddMany(ctx context.Context, nodes []Node) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, n := range nodes {
		d.nodes[n.Cid().KeyString()] = n
	}
	return nil
}

func (d *testDag) Remove(ctx context.Context, c *cid.Cid) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.nodes, c.KeyString())
	return nil
}

func (d *testDag) RemoveMany(ctx context.Context, cids []*cid.Cid) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, c := range cids {
		delete(d.nodes, c.KeyString())
	}
	return nil
}

var _ DAGService = new(testDag)

func TestBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := newTestDag()
	b := NewBatch(ctx, d)
	for i := 0; i < 1000; i++ {
		// It would be great if we could use *many* different nodes here
		// but we can't add any dependencies and I don't feel like adding
		// any more testing code.
		if err := b.Add(new(EmptyNode)); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Commit(); err != nil {
		t.Fatal(err)
	}

	n, err := d.Get(ctx, new(EmptyNode).Cid())
	if err != nil {
		t.Fatal(err)
	}
	switch n.(type) {
	case *EmptyNode:
	default:
		t.Fatal("expected the node to exist in the dag")
	}

	if len(d.nodes) != 1 {
		t.Fatal("should have one node")
	}
}
