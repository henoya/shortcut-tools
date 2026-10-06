package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/henoya/shortcut-tools/internal/defs"
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
//
// Cherri には常に --skip-sign を付けて未署名ファイルを作らせ、
// ソース中の @descriptor / @entity（sct defs が書く）から、Cherri が出せない情報を補う。
// そのあと macOS なら shortcuts sign で署名する。
func runCompile(args []string) error {
	fs := flag.NewFlagSet("compile", flag.ContinueOnError)
	noSign := fs.Bool("no-sign", false, "macOS でも署名しない")
	mode := fs.String("sign-mode", "people-who-know-me", "shortcuts sign の --mode（anyone / people-who-know-me）")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "使い方: sct compile [-no-sign] [-sign-mode M] <入力.cherri> [cherri のオプション...]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return flag.ErrHelp
	}
	in := fs.Arg(0)
	if hasSigningFlag(fs.Args()[1:]) {
		return errors.New("署名は sct compile が行うので、cherri の署名オプションは付けないでください")
	}
	metas, err := collectMeta(in, map[string]bool{})
	if err != nil {
		return err
	}

	dir := filepath.Dir(in)
	before := unsignedFiles(dir)
	if err := runCherri(append(fs.Args(), "--skip-sign")...); err != nil {
		return err
	}
	outs := changedFiles(dir, before)
	if len(outs) == 0 {
		return errors.New("cherri の出力ファイルが見つかりません")
	}
	for _, out := range outs {
		if err := patchMeta(out, metas); err != nil {
			return err
		}
		if runtime.GOOS == "darwin" && !*noSign {
			signed := strings.TrimSuffix(out, "_unsigned.shortcut") + ".shortcut"
			cmd := exec.Command("shortcuts", "sign", "--mode", *mode, "--input", out, "--output", signed)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("shortcuts sign に失敗しました: %w", err)
			}
			fmt.Fprintln(os.Stderr, "署名しました:", signed)
		} else {
			fmt.Fprintln(os.Stderr, "未署名で出力しました:", out)
		}
	}
	return nil
}

// collectMeta は入力ファイルと、そこから #include しているローカルのファイルから
// @descriptor / @entity を集める。
func collectMeta(path string, seen map[string]bool) (map[string]defs.Meta, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if seen[abs] {
		return nil, nil
	}
	seen[abs] = true
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	out, err := defs.ParseMeta(src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for _, m := range includeRe.FindAllSubmatch(src, -1) {
		inc := filepath.Join(filepath.Dir(abs), string(m[1]))
		if _, err := os.Stat(inc); err != nil {
			if _, err2 := os.Stat(inc + ".cherri"); err2 != nil {
				continue // Cherri 標準の include（actions/... など）
			}
			inc += ".cherri"
		}
		sub, err := collectMeta(inc, seen)
		if err != nil {
			return nil, err
		}
		for k, v := range sub {
			if _, ok := out[k]; !ok {
				out[k] = v
			}
		}
	}
	return out, nil
}

var includeRe = regexp.MustCompile(`(?m)^\s*#include\s+'([^']+)'`)

func patchMeta(path string, metas map[string]defs.Meta) error {
	if len(metas) == 0 {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	w, f, err := shortcut.Decode(b)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if defs.Apply(w, metas) == 0 {
		return nil
	}
	b, err = shortcut.Encode(w, f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// unsignedFiles は dir にある *_unsigned.shortcut と更新時刻を返す。
func unsignedFiles(dir string) map[string]time.Time {
	out := map[string]time.Time{}
	matches, _ := filepath.Glob(filepath.Join(dir, "*_unsigned.shortcut"))
	for _, m := range matches {
		if st, err := os.Stat(m); err == nil {
			out[m] = st.ModTime()
		}
	}
	return out
}

func changedFiles(dir string, before map[string]time.Time) []string {
	var out []string
	for path, mt := range unsignedFiles(dir) {
		if old, ok := before[path]; !ok || mt.After(old) {
			out = append(out, path)
		}
	}
	return out
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
