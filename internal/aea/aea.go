// Package aea は署名済みショートカット（Apple Encrypted Archive, プロファイル 0）から
// 中身の Apple Archive を取り出す。暗号化されていないので鍵は不要。
package aea

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/aixiansheng/lzfse"
)

// Magic は AEA ファイルの先頭 4 バイト。
var Magic = []byte("AEA1")

// 認証データの後ろ、圧縮ペイロードが始まるまでの固定長
// （ルートヘッダ・署名・クラスタヘッダなど）。libshortcutsign の実測値に合わせている。
const payloadOffsetAfterAuth = 0x495c - 0xc

// Archive は AEA から取り出した内容。
type Archive struct {
	// AuthData は署名証明書チェーンを含むバイナリ plist。
	AuthData []byte
	// Payload は LZFSE を展開した Apple Archive (AA01) のバイト列。
	Payload []byte
}

// IsAEA は b が AEA ファイルかどうかを返す。
func IsAEA(b []byte) bool {
	return len(b) >= 4 && bytes.Equal(b[:4], Magic)
}

// Open は署名済みショートカットのバイト列を解析する。
func Open(b []byte) (*Archive, error) {
	if !IsAEA(b) {
		return nil, errors.New("aea: AEA1 マジックがありません")
	}
	if len(b) < 12 {
		return nil, errors.New("aea: ヘッダが短すぎます")
	}
	authSize := int(binary.LittleEndian.Uint32(b[8:12]))
	if 12+authSize > len(b) {
		return nil, fmt.Errorf("aea: 認証データのサイズ %d がファイル長を超えています", authSize)
	}
	auth := b[12 : 12+authSize]

	start := 12 + authSize + payloadOffsetAfterAuth
	if start >= len(b) || !bytes.HasPrefix(b[start:], []byte("bvx")) {
		// 想定位置になければ認証データの後ろから LZFSE ブロックを探す。
		i := bytes.Index(b[12+authSize:], []byte("bvx2"))
		if i < 0 {
			return nil, errors.New("aea: LZFSE ペイロードが見つかりません")
		}
		start = 12 + authSize + i
	}

	payload, err := io.ReadAll(lzfse.NewReader(bytes.NewReader(b[start:])))
	if err != nil {
		return nil, fmt.Errorf("aea: LZFSE の展開に失敗しました: %w", err)
	}
	return &Archive{AuthData: auth, Payload: payload}, nil
}
