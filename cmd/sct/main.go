// sct はショートカット（Apple Shortcuts）のワークフローを扱う CLI。
package main

import (
	"flag"
	"fmt"
	"os"
)

const usage = `使い方: sct <コマンド> [オプション]

コマンド:
  convert   形式を変換する（署名済み .shortcut / binary / xml / json → binary / xml / json）
  show      ワークフローを読みやすく表示する
  decompile Cherri のコードに変換する（cherri が必要）
  compile   Cherri のコードからショートカットを作る（cherri が必要）

各コマンドの詳細は sct <コマンド> -h
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmds := map[string]func([]string) error{
		"convert":   runConvert,
		"show":      runShow,
		"decompile": runDecompile,
		"compile":   runCompile,
	}
	run, ok := cmds[os.Args[1]]
	if !ok {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := run(os.Args[2:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "sct:", err)
		os.Exit(1)
	}
}
