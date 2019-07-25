package main

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestPackUnpack(t *testing.T) {
	str := "HELLO WORLD FOR EVERYONE AND EVERYONE EVERYONE EVERYONE EVERYONE EVERYONE EVERYONE EVERYONE EVERYONE"
	buf := bytes.NewBufferString(str)
	packedr, err := pack(buf)
	ok(t, err)
	raw, err := ioutil.ReadAll(packedr)
	ok(t, err)
	assert(t, len(raw) < len(str), "expect gzipped verion to be less than original text")
	unpr, err := unpack(bytes.NewBuffer(raw))
	ok(t, err)
	rawstr, err := ioutil.ReadAll(unpr)
	ok(t, err)
	equals(t, str, string(rawstr))
}
