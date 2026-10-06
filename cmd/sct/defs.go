package main

import (
	"bytes"
	"flag"
	"fmt"

	"github.com/henoya/shortcut-tools/internal/defs"
	"github.com/henoya/shortcut-tools/internal/shortcut"
)

// runDefs はショートカットに含まれるアプリ提供アクションの Cherri 定義を書き出す。
func runDefs(args []string) error {
	fs := flag.NewFlagSet("defs", flag.ContinueOnError)
	out := fs.String("o", "-", "出力ファイル（- で標準出力）")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "使い方: sct defs [-o 出力.cherri] <入力>...")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return flag.ErrHelp
	}
	var ws []shortcut.Workflow
	for _, path := range fs.Args() {
		w, _, err := load(path)
		if err != nil {
			return err
		}
		ws = append(ws, w)
	}
	var b bytes.Buffer
	if err := defs.Write(&b, defs.Collect(ws...)); err != nil {
		return err
	}
	return writeOut(*out, b.Bytes())
}
