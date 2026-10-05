package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/henoya/shortcut-tools/internal/view"
)

func runShow(args []string) error {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	maxValue := fs.Int("max", 60, "値を表示する最大文字数（0 で全部）")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "使い方: sct show [-max N] <入力>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return flag.ErrHelp
	}
	w, _, err := load(fs.Arg(0))
	if err != nil {
		return err
	}
	return view.Render(os.Stdout, w, view.Options{MaxValue: *maxValue})
}
