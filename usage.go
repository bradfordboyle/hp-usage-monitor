package main

import (
	"io"
	"strconv"
	"strings"

	"gopkg.in/xmlpath.v2"
)

type usage struct {
	Root *xmlpath.Node
}

func newUsage(r io.Reader) *usage {
	n, err := xmlpath.ParseHTML(r)
	if err != nil {
		logger.Fatal(err)
	}
	u := &usage{Root: n}

	return u
}

func (u *usage) simplex() uint64 {
	return u.getPaperUsage("//*[@id=\"tbl-1851\"]/tbody/tr[2]/td[2]/div")
}

func (u *usage) duplex() uint64 {
	return u.getPaperUsage("//*[@id=\"tbl-1851\"]/tbody/tr[2]/td[3]/div")
}

func (u *usage) getPaperUsage(s string) uint64 {
	path := xmlpath.MustCompile(s)
	value, _ := path.String(u.Root)
	value = strings.Replace(value, ",", "", -1)
	rv, _ := strconv.ParseUint(value, 10, 64)
	return rv
}
