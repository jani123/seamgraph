package npc

import w "example.test/architecture/world"
func Read(r w.Reader) string { return r.Read() }
func New() w.Reader { return w.Open() }
