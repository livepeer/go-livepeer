package cmds

import (
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

type Foo struct {
	Bar int
}

type Bar struct {
	Foo string
}

type ValueError struct {
	Error error
	Value interface{}
}

type anyTestCase struct {
	Value   interface{}
	JSON    string
	Decoded []ValueError
}

func TestMaybeError(t *testing.T) {
	testcases := []anyTestCase{
		{
			Value: &Foo{},
			JSON:  `{"Bar":23}{"Bar":42}{"Message":"some error", "Type": "error"}`,
			Decoded: []ValueError{
				ValueError{Error: nil, Value: &Foo{23}},
				ValueError{Error: nil, Value: &Foo{42}},
				ValueError{Error: nil, Value: cmdkit.Error{Message: "some error", Code: 0}},
			},
		},
		{
			Value: Foo{},
			JSON:  `{"Bar":23}{"Bar":42}{"Message":"some error", "Type": "error"}`,
			Decoded: []ValueError{
				ValueError{Error: nil, Value: &Foo{23}},
				ValueError{Error: nil, Value: &Foo{42}},
				ValueError{Error: nil, Value: cmdkit.Error{Message: "some error", Code: 0}},
			},
		},
		{
			Value: &Bar{},
			JSON:  `{"Foo":""}{"Foo":"Qmabc"}{"Message":"some error", "Type": "error"}`,
			Decoded: []ValueError{
				ValueError{Error: nil, Value: &Bar{""}},
				ValueError{Error: nil, Value: &Bar{"Qmabc"}},
				ValueError{Error: nil, Value: cmdkit.Error{Message: "some error", Code: 0}},
			},
		},
		{
			Value: Bar{},
			JSON:  `{"Foo":""}{"Foo":"Qmabc"}{"Message":"some error", "Type": "error"}`,
			Decoded: []ValueError{
				ValueError{Error: nil, Value: &Bar{""}},
				ValueError{Error: nil, Value: &Bar{"Qmabc"}},
				ValueError{Error: nil, Value: cmdkit.Error{Message: "some error", Code: 0}},
			},
		},
		{
			JSON: `{"Foo":"bar", "i": 4}"some string"5{"Message":"some error", "Type": "error"}`,
			Decoded: []ValueError{
				ValueError{Error: nil, Value: map[string]interface{}{"Foo": "bar", "i": 4.0}},
				ValueError{Error: nil, Value: "some string"},
				ValueError{Error: nil, Value: 5.0},
				ValueError{Error: nil, Value: cmdkit.Error{Message: "some error", Code: 0}},
			},
		},
	}

	for _, tc := range testcases {

		r := strings.NewReader(tc.JSON)
		d := json.NewDecoder(r)

		var err error

		for _, dec := range tc.Decoded {
			m := &MaybeError{Value: tc.Value}

			err = d.Decode(m)
			if err != dec.Error {
				t.Fatalf("error is %v, expected %v", err, dec.Error)
			}

			rx := m.Get()
			ex := dec.Value

			if !reflect.DeepEqual(ex, rx) {
				t.Errorf("value is %#v(%T), expected %#v(%T)", rx, rx, ex, ex)
			}
		}

		m := &MaybeError{Value: tc.Value}
		err = d.Decode(m)
		if err != io.EOF {
			t.Fatal("data left in decoder:", m.Get())
		}
	}
}
