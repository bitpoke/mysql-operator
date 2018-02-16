package strings_test

import (
	"fmt"
	"testing"

	. "github.com/appscode/go/strings"
)

const testStr = `
Quite
things
in
a
row
and

things

not

in

a
*******************************************
********************************************

**************************************
  *************
  ******** ******  ************   **************






***************************************************
row

and

things



that




are



too





far
end`

func TestFmt(t *testing.T) {
	ans := Fmt(testStr)
	fmt.Println(ans)
}
