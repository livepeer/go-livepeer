package shell

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/cheekybits/is"
)

const (
	examplesHash = "QmVtU7ths96fMgZ8YSZAbKghyieq7AjxNdcqyVzxTt3qVe"
	shellUrl     = "localhost:5001"
)

func TestAdd(t *testing.T) {
	is := is.New(t)
	s := NewShell(shellUrl)

	mhash, err := s.Add(bytes.NewBufferString("Hello IPFS Shell tests"))
	is.Nil(err)
	is.Equal(mhash, "QmUfZ9rAdhV5ioBzXKdUTh2ZNsz9bzbkaLVyQ8uc8pj21F")
}

func TestLocalShell(t *testing.T) {
	is := is.New(t)
	s := NewLocalShell()
	is.NotNil(s)

	mhash, err := s.Add(bytes.NewBufferString("Hello IPFS Shell tests"))
	is.Nil(err)
	is.Equal(mhash, "QmUfZ9rAdhV5ioBzXKdUTh2ZNsz9bzbkaLVyQ8uc8pj21F")
}

func TestCat(t *testing.T) {
	is := is.New(t)
	s := NewShell(shellUrl)

	rc, err := s.Cat(fmt.Sprintf("/ipfs/%s/readme", examplesHash))
	is.Nil(err)

	md5 := md5.New()
	_, err = io.Copy(md5, rc)
	is.Nil(err)
	is.Equal(fmt.Sprintf("%x", md5.Sum(nil)), "3fdcaad186e79983a6920b4c7eeda949")
}

func TestList(t *testing.T) {
	is := is.New(t)
	s := NewShell(shellUrl)

	list, err := s.List(fmt.Sprintf("/ipfs/%s", examplesHash))
	is.Nil(err)

	is.Equal(len(list), 6)

	// TODO: document difference in size between 'ipfs ls' and 'ipfs file ls -v'. additional object encoding in data block?
	expected := map[string]LsLink{
		"about":          {Type: TFile, Hash: "QmZTR5bcpQD7cFgTorqxZDYaew1Wqgfbd2ud9QqGPAkK2V", Name: "about", Size: 1688},
		"contact":        {Type: TFile, Hash: "QmYCvbfNbCwFR45HiNP45rwJgvatpiW38D961L5qAhUM5Y", Name: "contact", Size: 200},
		"help":           {Type: TFile, Hash: "QmY5heUM5qgRubMDD1og9fhCPA6QdkMp3QCwd4s7gJsyE7", Name: "help", Size: 322},
		"quick-start":    {Type: TFile, Hash: "QmUzLxaXnM8RYCPEqLDX5foToi5aNZHqfYr285w2BKhkft", Name: "quick-start", Size: 1697},
		"readme":         {Type: TFile, Hash: "QmPZ9gcCEpqKTo6aq61g2nXGUhM4iCL3ewB6LDXZCtioEB", Name: "readme", Size: 1102},
		"security-notes": {Type: TFile, Hash: "QmTumTjvcYCAvRRwQ8sDRxh8ezmrcr88YFU7iYNroGGTBZ", Name: "security-notes", Size: 1027},
	}
	for _, l := range list {
		el, ok := expected[l.Name]
		is.True(ok)
		is.NotNil(el)
		is.Equal(*l, el)
	}
}

func TestFileList(t *testing.T) {
	is := is.New(t)
	s := NewShell(shellUrl)

	list, err := s.FileList(fmt.Sprintf("/ipfs/%s", examplesHash))
	is.Nil(err)

	is.Equal(list.Type, "Directory")
	is.Equal(list.Size, 0)
	is.Equal(len(list.Links), 6)

	// TODO: document difference in sice betwen 'ipfs ls' and 'ipfs file ls -v'. additional object encoding in data block?
	expected := map[string]UnixLsLink{
		"about":          {Type: "File", Hash: "QmZTR5bcpQD7cFgTorqxZDYaew1Wqgfbd2ud9QqGPAkK2V", Name: "about", Size: 1677},
		"contact":        {Type: "File", Hash: "QmYCvbfNbCwFR45HiNP45rwJgvatpiW38D961L5qAhUM5Y", Name: "contact", Size: 189},
		"help":           {Type: "File", Hash: "QmY5heUM5qgRubMDD1og9fhCPA6QdkMp3QCwd4s7gJsyE7", Name: "help", Size: 311},
		"quick-start":    {Type: "File", Hash: "QmUzLxaXnM8RYCPEqLDX5foToi5aNZHqfYr285w2BKhkft", Name: "quick-start", Size: 1686},
		"readme":         {Type: "File", Hash: "QmPZ9gcCEpqKTo6aq61g2nXGUhM4iCL3ewB6LDXZCtioEB", Name: "readme", Size: 1091},
		"security-notes": {Type: "File", Hash: "QmTumTjvcYCAvRRwQ8sDRxh8ezmrcr88YFU7iYNroGGTBZ", Name: "security-notes", Size: 1016},
	}
	for _, l := range list.Links {
		el, ok := expected[l.Name]
		is.True(ok)
		is.NotNil(el)
		is.Equal(*l, el)
	}
}

func TestPins(t *testing.T) {
	is := is.New(t)
	s := NewShell(shellUrl)

	// Add a thing, which pins it by default
	h, err := s.Add(bytes.NewBufferString("go-ipfs-api pins test 9F3D1F30-D12A-4024-9477-8F0C8E4B3A63"))
	is.Nil(err)

	pins, err := s.Pins()
	is.Nil(err)

	_, ok := pins[h]
	is.True(ok)

	err = s.Unpin(h)
	is.Nil(err)

	pins, err = s.Pins()
	is.Nil(err)

	_, ok = pins[h]
	is.False(ok)

	err = s.Pin(h)
	is.Nil(err)

	pins, err = s.Pins()
	is.Nil(err)

	info, ok := pins[h]
	is.True(ok)
	is.Equal(info.Type, RecursivePin)
}

func TestPatch_rmLink(t *testing.T) {
	is := is.New(t)
	s := NewShell(shellUrl)
	newRoot, err := s.Patch(examplesHash, "rm-link", "about")
	is.Nil(err)
	is.Equal(newRoot, "QmNjJ3naRhHCn14E895R1xtGmDgKQb8vnVvQar6RrnraC1")
}

func TestPatchLink(t *testing.T) {
	is := is.New(t)
	s := NewShell(shellUrl)

	newRoot, err := s.PatchLink(examplesHash, "about", "QmUXTtySmd7LD4p6RG6rZW6RuUuPZXTtNMmRQ6DSQo3aMw", true)
	is.Nil(err)
	is.Equal(newRoot, "QmQwWjFnEPxwmkb5Ukn6UnbrBVebSAYnM11nmMs89e7zH9")
}

func TestResolvePath(t *testing.T) {
	is := is.New(t)
	s := NewShell(shellUrl)

	childHash, err := s.ResolvePath(fmt.Sprintf("/ipfs/%s/about", examplesHash))
	is.Nil(err)
	is.Equal(childHash, "QmZTR5bcpQD7cFgTorqxZDYaew1Wqgfbd2ud9QqGPAkK2V")
}

func TestPubSub(t *testing.T) {
	is := is.New(t)
	s := NewShell(shellUrl)

	var (
		topic = "test"

		sub *PubSubSubscription
		err error
	)

	t.Log("subscribing...")
	sub, err = s.PubSubSubscribe(topic)
	is.Nil(err)
	is.NotNil(sub)
	t.Log("sub: done")

	time.Sleep(10 * time.Millisecond)

	t.Log("publishing...")
	is.Nil(s.PubSubPublish(topic, "Hello World!"))
	t.Log("pub: done")

	t.Log("next()...")
	r, err := sub.Next()
	t.Log("next: done. ")

	is.Nil(err)
	is.NotNil(r)
	is.Equal(r.Data(), "Hello World!")

	sub2, err := s.PubSubSubscribe(topic)
	is.Nil(err)
	is.NotNil(sub2)

	is.Nil(s.PubSubPublish(topic, "Hallo Welt!"))

	r, err = sub2.Next()
	is.Nil(err)
	is.NotNil(r)
	is.Equal(r.Data(), "Hallo Welt!")

	r, err = sub.Next()
	is.NotNil(r)
	is.Nil(err)
	is.Equal(r.Data(), "Hallo Welt!")

	is.Nil(sub.Cancel())
}

func TestObjectStat(t *testing.T) {
	obj := "QmZTR5bcpQD7cFgTorqxZDYaew1Wqgfbd2ud9QqGPAkK2V"
	is := is.New(t)
	s := NewShell(shellUrl)
	stat, err := s.ObjectStat("QmZTR5bcpQD7cFgTorqxZDYaew1Wqgfbd2ud9QqGPAkK2V")
	is.Nil(err)
	is.Equal(stat.Hash, obj)
	is.Equal(stat.LinksSize, 3)
	is.Equal(stat.CumulativeSize, 1688)
}
