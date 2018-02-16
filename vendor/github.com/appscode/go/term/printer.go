package term

import (
	"fmt"
	"os"

	"github.com/appscode/go/env"
	"github.com/appscode/go/errors"
	"github.com/fatih/color"
)

func Print(args ...interface{}) {
	fmt.Print(args...)
}

func Println(args ...interface{}) {
	fmt.Println(args...)
}

func Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func Infoln(i ...interface{}) {
	doPrintln([]color.Attribute{color.FgCyan}, i)
}

func Warningln(i ...interface{}) {
	doPrintln([]color.Attribute{color.FgYellow}, i)
}

func Successln(i ...interface{}) {
	doPrintln([]color.Attribute{color.FgGreen}, i)
}

func Errorln(i ...interface{}) {
	doPrintln([]color.Attribute{color.FgRed, color.Bold}, i)
}

func Fatalln(i ...interface{}) {
	doPrintln([]color.Attribute{color.FgRed, color.Bold}, i)
	os.Exit(1)
}

func ExitOnError(err error) {
	if err != nil {
		doPrintln([]color.Attribute{color.FgRed, color.Bold}, []interface{}{err.Error()})
		if Env != env.Prod {
			fmt.Fprintln(os.Stderr, errors.FromErr(err).Error())
		}
		os.Exit(1)
	}
}

func doPrint(c []color.Attribute, args []interface{}) {
	if Interactive && len(c) > 0 {
		color.Set(c...)
		defer color.Set(color.Reset)
	}
	fmt.Print(args...)
}

func doPrintln(c []color.Attribute, args []interface{}) {
	if Interactive && len(c) > 0 {
		color.Set(c...)
		defer color.Set(color.Reset)
	}
	fmt.Println(args...)
}

func doPrintf(c []color.Attribute, format string, args []interface{}) {
	if Interactive && len(c) > 0 {
		color.Set(c...)
		defer color.Set(color.Reset)
	}
	fmt.Printf(format, args...)
}
