// Package aar は Apple Archive (AA01) 形式を読む最小限の実装。
package aar

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

var magic = []byte("AA01")

// Entry はアーカイブ内の 1 項目。
type Entry struct {
	Type byte   // 'F' ファイル, 'D' ディレクトリなど
	Path string // PAT フィールド
	Data []byte // DAT フィールドのブロブ
}

// Parse は AA01 のバイト列を項目の一覧にする。
func Parse(b []byte) ([]Entry, error) {
	var entries []Entry
	for len(b) > 0 {
		if len(b) < 6 || !bytes.Equal(b[:4], magic) {
			return nil, errors.New("aar: AA01 ヘッダがありません")
		}
		hsize := int(binary.LittleEndian.Uint16(b[4:6]))
		if hsize < 6 || hsize > len(b) {
			return nil, fmt.Errorf("aar: ヘッダサイズ %d が不正です", hsize)
		}
		e, blobSizes, err := parseFields(b[6:hsize])
		if err != nil {
			return nil, err
		}
		b = b[hsize:]
		// ブロブはヘッダの直後にフィールドの順で並ぶ。
		for _, bs := range blobSizes {
			if bs.size > uint64(len(b)) {
				return nil, fmt.Errorf("aar: %s のブロブがアーカイブ長を超えています", bs.key)
			}
			if bs.key == "DAT" {
				e.Data = b[:bs.size]
			}
			b = b[bs.size:]
		}
		entries = append(entries, e)
	}
	return entries, nil
}

type blob struct {
	key  string
	size uint64
}

func parseFields(h []byte) (Entry, []blob, error) {
	var e Entry
	var blobs []blob
	for len(h) > 0 {
		if len(h) < 4 {
			return e, nil, errors.New("aar: フィールドが途中で切れています")
		}
		key, sub := string(h[:3]), h[3]
		h = h[4:]
		n, val, isBlob, err := fieldValue(sub, h)
		if err != nil {
			return e, nil, fmt.Errorf("aar: %s: %w", key, err)
		}
		switch {
		case isBlob:
			blobs = append(blobs, blob{key, val})
		case key == "TYP":
			e.Type = byte(val)
		case key == "PAT":
			e.Path = string(h[2:n])
		}
		h = h[n:]
	}
	return e, blobs, nil
}

// fieldValue はサブタイプからフィールド値の長さを求める。
// 返り値: 消費バイト数, 整数値（ブロブならサイズ）, ブロブかどうか。
func fieldValue(sub byte, h []byte) (int, uint64, bool, error) {
	need := func(n int) error {
		if len(h) < n {
			return errors.New("値が途中で切れています")
		}
		return nil
	}
	le := func(n int) uint64 {
		var v uint64
		for i := n - 1; i >= 0; i-- {
			v = v<<8 | uint64(h[i])
		}
		return v
	}
	switch sub {
	case '*': // フラグのみ
		return 0, 0, false, nil
	case '1', '2', '4', '8': // 整数
		n := int(sub - '0')
		if err := need(n); err != nil {
			return 0, 0, false, err
		}
		return n, le(n), false, nil
	case 'P': // 長さ付き文字列
		if err := need(2); err != nil {
			return 0, 0, false, err
		}
		n := 2 + int(le(2))
		return n, 0, false, need(n)
	case 'A', 'B', 'C': // ブロブ（サイズ 2/4/8 バイト）
		n := map[byte]int{'A': 2, 'B': 4, 'C': 8}[sub]
		if err := need(n); err != nil {
			return 0, 0, false, err
		}
		return n, le(n), true, nil
	case 'S': // タイムスタンプ（秒のみ）
		return 8, 0, false, need(8)
	case 'T': // タイムスタンプ（秒 + ナノ秒）
		return 12, 0, false, need(12)
	case 'F', 'G', 'H', 'I', 'J': // ハッシュ（CRC32/SHA1/SHA256/SHA384/SHA512）
		n := map[byte]int{'F': 4, 'G': 20, 'H': 32, 'I': 48, 'J': 64}[sub]
		return n, 0, false, need(n)
	}
	return 0, 0, false, fmt.Errorf("未知のサブタイプ %q", sub)
}
