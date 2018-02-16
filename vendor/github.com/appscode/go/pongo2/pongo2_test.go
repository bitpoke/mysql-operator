package pongo2

import (
	"fmt"
	"testing"

	p "github.com/flosch/pongo2"
)

func TestViaJson(t *testing.T) {
	s := struct {
		aString string
		aBool   bool
	}{
		"name", true,
	}
	fmt.Println(s)
	ptx, err := ViaJson(&s)
	if err != nil {
		t.Fatal(err)
	}

	tpl, err := p.FromString("aString: {{ aString }} aBool: {{ aBool }}")
	if err != nil {
		t.Fatal(err)
	}
	out, err := tpl.Execute(ptx)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(out) // Output: Hello Flori
}
