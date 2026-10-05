package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/henoya/shortcut-tools/internal/shortcut"
)

// cherriPath は使う cherri コマンド。環境変数 CHERRI で上書きできる。
func cherriPath() string {
	if p := os.Getenv("CHERRI"); p != "" {
		return p
	}
	return "cherri"
}

func runCherri(args ...string) error {
	cmd := exec.Command(cherriPath(), args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cherri の実行に失敗しました: %w", err)
	}
	return nil
}

// runDecompile は署名済みを含むショートカットを Cherri のコードにする。
// Cherri 自体は署名済みファイルを読めないので、ここで plist に展開してから渡す。
func runDecompile(args []string) error {
	fs := flag.NewFlagSet("decompile", flag.ContinueOnError)
	out := fs.String("o", "", "出力する .cherri ファイル（省略時は入力と同じ場所）")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "使い方: sct decompile [-o 出力.cherri] <入力>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return flag.ErrHelp
	}
	in := fs.Arg(0)
	w, _, err := load(in)
	if err != nil {
		return err
	}
	base := strings.TrimSuffix(filepath.Base(in), filepath.Ext(in))
	if *out == "" {
		*out = filepath.Join(filepath.Dir(in), base+".cherri")
	}

	tmp, err := os.MkdirTemp("", "sct-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	pl, err := shortcut.Encode(w, shortcut.FormatXML)
	if err != nil {
		return err
	}
	tmpPlist := filepath.Join(tmp, base+".plist")
	if err := os.WriteFile(tmpPlist, pl, 0o600); err != nil {
		return err
	}
	abs, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	return runCherri("--import="+tmpPlist, "--output="+abs, "--no-ansi")
}

// runCompile は Cherri のコードからショートカットを作る。
// macOS 以外では署名できないので、署名方法の指定がなければ --skip-sign を付ける。
func runCompile(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintln(os.Stderr, "使い方: sct compile <入力.cherri> [cherri のオプション...]")
		return flag.ErrHelp
	}
	if runtime.GOOS != "darwin" && !hasSigningFlag(args) {
		args = append(args, "--skip-sign")
	}
	return runCherri(args...)
}

func hasSigningFlag(args []string) bool {
	for _, a := range args {
		for _, f := range []string{"--skip-sign", "--hubsign", "--signing-server"} {
			if strings.HasPrefix(a, f) {
				return true
			}
		}
	}
	return false
}
