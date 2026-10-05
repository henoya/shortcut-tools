package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/henoya/shortcut-tools/internal/shortcut"
)

func runConvert(args []string) error {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	to := fs.String("to", "xml", "出力形式: xml, binary, json")
	out := fs.String("o", "-", "出力ファイル（- で標準出力）")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "使い方: sct convert [-to xml|binary|json] [-o 出力] <入力>")
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
	b, err := shortcut.Encode(w, shortcut.Format(*to))
	if err != nil {
		return err
	}
	return writeOut(*out, b)
}

// load はファイル（- で標準入力）からワークフローを読み込む。
func load(path string) (shortcut.Workflow, shortcut.Format, error) {
	var b []byte
	var err error
	if path == "-" {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, "", err
	}
	w, f, err := shortcut.Decode(b)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", path, err)
	}
	return w, f, nil
}

func writeOut(path string, b []byte) error {
	if path == "-" {
		_, err := os.Stdout.Write(b)
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
