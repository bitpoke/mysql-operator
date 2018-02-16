package term

// A /dev/null logger for GRPC
type NullLogger struct{}

func (g *NullLogger) Fatal(args ...interface{}) {
}

func (g *NullLogger) Fatalf(format string, args ...interface{}) {
}

func (g *NullLogger) Fatalln(args ...interface{}) {
}

func (g *NullLogger) Print(args ...interface{}) {
}

func (g *NullLogger) Printf(format string, args ...interface{}) {
}

func (g *NullLogger) Println(args ...interface{}) {
}
