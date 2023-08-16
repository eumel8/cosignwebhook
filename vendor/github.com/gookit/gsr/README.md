# GSR

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/gookit/gsr?style=flat-square)
[![GoDoc](https://godoc.org/github.com/gookit/gsr?status.svg)](https://pkg.go.dev/github.com/gookit/gsr)
[![Go Report Card](https://goreportcard.com/badge/github.com/gookit/gsr)](https://goreportcard.com/report/github.com/gookit/gsr)

Go Standards Recommendations

## Install

```bash
go get github.com/gookit/gsr
```

## Interfaces

- [logger](logger.go)
- [cacher](cacher.go)
- [parser](parser.go)

## Usage

### Logger Interface

**Std Logger**

```go
package main
import (
	"github.com/gookit/gsr"
)

type MyApp struct {
	logger gsr.StdLogger // std logger
}

func (ma *MyApp) SetLogger(logger gsr.StdLogger)  {
	ma.logger = logger
}
```

**Full Logger**

```go
package main
import (
	"github.com/gookit/gsr"
)

type MyApp struct {
	logger gsr.Logger // full logger
}

func (ma *MyApp) SetLogger(logger gsr.Logger)  {
	ma.logger = logger
}
```

### Cache Interface

**Simple Cache**

```go
package main
import (
	"github.com/gookit/gsr"
)

type MyApp struct {
	cacher gsr.SimpleCacher
}

func (ma *MyApp) SetCacher(cacher gsr.SimpleCacher)  {
	ma.cacher = cacher
}
```

### DataParser interface

```go
// DataParser interface for Marshal/Unmarshal data
type DataParser interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, ptr any) error
}
```

## LICENSE

[MIT](LICENSE)
