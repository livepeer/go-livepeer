package main

import (
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/ipfs/go-ipfs-api"

	u "github.com/ipfs/go-ipfs-util"
)

var sh *shell.Shell
var ncalls int

var _ = time.ANSIC

func sleep() {
	ncalls++
	//time.Sleep(time.Millisecond * 5)
}

func randString() string {
	alpha := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
	l := rand.Intn(10) + 2

	var s string
	for i := 0; i < l; i++ {
		s += string([]byte{alpha[rand.Intn(len(alpha))]})
	}
	return s
}

func makeRandomObject() (string, error) {
	// do some math to make a size
	x := rand.Intn(120) + 1
	y := rand.Intn(120) + 1
	z := rand.Intn(120) + 1
	size := x * y * z

	r := io.LimitReader(u.NewTimeSeededRand(), int64(size))
	sleep()
	return sh.Add(r)
}

func makeRandomDir(depth int) (string, error) {
	if depth <= 0 {
		return makeRandomObject()
	}
	sleep()
	empty, err := sh.NewObject("unixfs-dir")
	if err != nil {
		return "", err
	}

	curdir := empty
	for i := 0; i < rand.Intn(8)+2; i++ {
		var obj string
		if rand.Intn(2) == 1 {
			obj, err = makeRandomObject()
			if err != nil {
				return "", err
			}
		} else {
			obj, err = makeRandomDir(depth - 1)
			if err != nil {
				return "", err
			}
		}

		name := randString()
		sleep()
		nobj, err := sh.PatchLink(curdir, name, obj, true)
		if err != nil {
			return "", err
		}
		curdir = nobj
	}

	return curdir, nil
}

func main() {
	sh = shell.NewShell("localhost:5001")
	for i := 0; i < 200; i++ {
		_, err := makeRandomObject()
		if err != nil {
			fmt.Println("err: ", err)
			return
		}
	}
	fmt.Println("we're okay")

	out, err := makeRandomDir(10)
	fmt.Printf("%d calls\n", ncalls)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(out)
	for {
		time.Sleep(time.Second * 1000)
	}
}
