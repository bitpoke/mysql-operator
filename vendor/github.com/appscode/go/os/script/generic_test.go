package script

import (
	"fmt"
	"os/user"
	"reflect"
	"testing"
)

func TestUserExists(t *testing.T) {
	s := &LinuxGeneric{}
	s.Init("/")
	fmt.Println(" EXISTS = ", s.UserExists("sanjid2"))

	if _, err := user.Lookup("sanjid2"); err != nil {

		fmt.Println(reflect.TypeOf(err))
		fmt.Println(err)

		_, ok := err.(user.UnknownUserError)

		fmt.Println(ok)
		//; ok {
		//	return false
		//}
		//errorutil.EOE(err)
	}
	//return true
}
