package gsr

// Printer interface definition
type Printer interface {
	Print(v ...any)
	Printf(format string, v ...any)
	Println(v ...any)
}

// StdLogger interface definition. refer the go "log" package.
type StdLogger interface {
	Printer
	Fatal(v ...any)
	Fatalf(format string, v ...any)
	Fatalln(v ...any)
	Panic(v ...any)
	Panicf(format string, v ...any)
	Panicln(v ...any)
}

// GenLogger generic logger interface definition
type GenLogger interface {
	Debug(v ...any)
	Debugf(format string, v ...any)
	Info(v ...any)
	Infof(format string, v ...any)
	Warn(v ...any)
	Warnf(format string, v ...any)
	Error(v ...any)
	Errorf(format string, v ...any)
}

// Logger interface definition
type Logger interface {
	StdLogger
	GenLogger
}
