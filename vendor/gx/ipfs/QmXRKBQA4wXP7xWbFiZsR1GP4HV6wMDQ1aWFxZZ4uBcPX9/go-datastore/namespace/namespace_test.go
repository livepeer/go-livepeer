package namespace_test

import (
	"bytes"
	"sort"
	"testing"

	. "gx/ipfs/QmYBJ8BXPDTMnzLFdv4rS5kbR1fUFASDVDpK7ZbeWMx6hq/go-check"

	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	ns "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore/namespace"
	dsq "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore/query"
	dstest "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore/test"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type DSSuite struct{}

var _ = Suite(&DSSuite{})

func (ks *DSSuite) TestBasic(c *C) {
	ks.testBasic(c, "abc")
	ks.testBasic(c, "")
}

func (ks *DSSuite) testBasic(c *C, prefix string) {

	mpds := ds.NewMapDatastore()
	nsds := ns.Wrap(mpds, ds.NewKey(prefix))

	keys := strsToKeys([]string{
		"foo",
		"foo/bar",
		"foo/bar/baz",
		"foo/barb",
		"foo/bar/bazb",
		"foo/bar/baz/barb",
	})

	for _, k := range keys {
		err := nsds.Put(k, []byte(k.String()))
		c.Check(err, Equals, nil)
	}

	for _, k := range keys {
		v1, err := nsds.Get(k)
		c.Check(err, Equals, nil)
		c.Check(bytes.Equal(v1.([]byte), []byte(k.String())), Equals, true)

		v2, err := mpds.Get(ds.NewKey(prefix).Child(k))
		c.Check(err, Equals, nil)
		c.Check(bytes.Equal(v2.([]byte), []byte(k.String())), Equals, true)
	}

	run := func(d ds.Datastore, q dsq.Query) []ds.Key {
		r, err := d.Query(q)
		c.Check(err, Equals, nil)

		e, err := r.Rest()
		c.Check(err, Equals, nil)

		return ds.EntryKeys(e)
	}

	listA := run(mpds, dsq.Query{})
	listB := run(nsds, dsq.Query{})
	c.Check(len(listA), Equals, len(listB))

	// sort them cause yeah.
	sort.Sort(ds.KeySlice(listA))
	sort.Sort(ds.KeySlice(listB))

	for i, kA := range listA {
		kB := listB[i]
		c.Check(nsds.InvertKey(kA), Equals, kB)
		c.Check(kA, Equals, nsds.ConvertKey(kB))
	}
}

func (ks *DSSuite) TestQuery(c *C) {
	mpds := dstest.NewTestDatastore(true)
	nsds := ns.Wrap(mpds, ds.NewKey("/foo"))

	keys := strsToKeys([]string{
		"abc/foo",
		"bar/foo",
		"foo/bar",
		"foo/bar/baz",
		"foo/baz/abc",
		"xyz/foo",
	})

	for _, k := range keys {
		err := mpds.Put(k, []byte(k.String()))
		c.Check(err, Equals, nil)
	}

	qres, err := nsds.Query(dsq.Query{})
	c.Check(err, Equals, nil)

	expect := []dsq.Entry{
		{Key: "/bar", Value: []byte("/foo/bar")},
		{Key: "/bar/baz", Value: []byte("/foo/bar/baz")},
		{Key: "/baz/abc", Value: []byte("/foo/baz/abc")},
	}

	results, err := qres.Rest()
	c.Check(err, Equals, nil)
	sort.Slice(results, func(i, j int) bool { return results[i].Key < results[j].Key })

	for i, ent := range results {
		c.Check(ent.Key, Equals, expect[i].Key)
		entval, _ := ent.Value.([]byte)
		expval, _ := expect[i].Value.([]byte)
		c.Check(string(entval), Equals, string(expval))
	}

	err = qres.Close()
	c.Check(err, Equals, nil)

	qres, err = nsds.Query(dsq.Query{Prefix: "bar"})
	c.Check(err, Equals, nil)

	expect = []dsq.Entry{
		{Key: "/bar", Value: []byte("/foo/bar")},
		{Key: "/bar/baz", Value: []byte("/foo/bar/baz")},
	}

	results, err = qres.Rest()
	c.Check(err, Equals, nil)
	sort.Slice(results, func(i, j int) bool { return results[i].Key < results[j].Key })

	for i, ent := range results {
		c.Check(ent.Key, Equals, expect[i].Key)
		entval, _ := ent.Value.([]byte)
		expval, _ := expect[i].Value.([]byte)
		c.Check(string(entval), Equals, string(expval))
	}

	if err := nsds.Datastore.(ds.CheckedDatastore).Check(); err != dstest.TestError {
		c.Errorf("Unexpected Check() error: %s", err)
	}

	if err := nsds.Datastore.(ds.GCDatastore).CollectGarbage(); err != dstest.TestError {
		c.Errorf("Unexpected CollectGarbage() error: %s", err)
	}

	if err := nsds.Datastore.(ds.ScrubbedDatastore).Scrub(); err != dstest.TestError {
		c.Errorf("Unexpected Scrub() error: %s", err)
	}
}

func strsToKeys(strs []string) []ds.Key {
	keys := make([]ds.Key, len(strs))
	for i, s := range strs {
		keys[i] = ds.NewKey(s)
	}
	return keys
}
