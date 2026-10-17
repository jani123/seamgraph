package world

type Reader interface { Read() string }
type Concrete struct{}
func (Concrete) Read() string { return "world" }
func Open() Reader { return Concrete{} }
