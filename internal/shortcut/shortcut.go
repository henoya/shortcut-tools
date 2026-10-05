// Package shortcut はショートカットのワークフローを読み込み、各形式へ書き出す。
//
// 対応する入力: 署名済み .shortcut (AEA)、バイナリ plist、XML plist、JSON。
package shortcut

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/henoya/shortcut-tools/internal/aar"
	"github.com/henoya/shortcut-tools/internal/aea"
	"howett.net/plist"
)

// Format はワークフローの保存形式。
type Format string

const (
	FormatSigned Format = "signed" // AEA 署名済み（読み込みのみ）
	FormatBinary Format = "binary" // バイナリ plist
	FormatXML    Format = "xml"    // XML plist
	FormatJSON   Format = "json"   // JSON（本ツールの表現）
)

// Workflow はショートカットの中身。plist のルート辞書をそのまま持つ。
type Workflow map[string]any

// Decode はバイト列の形式を判定してワークフローを読み込む。
func Decode(b []byte) (Workflow, Format, error) {
	if aea.IsAEA(b) {
		pl, err := Unsign(b)
		if err != nil {
			return nil, "", err
		}
		w, _, err := Decode(pl)
		return w, FormatSigned, err
	}
	if t := bytes.TrimLeft(b, " \t\r\n\ufeff"); len(t) > 0 && t[0] == '{' {
		w, err := decodeJSON(b)
		return w, FormatJSON, err
	}
	var w Workflow
	f, err := plist.Unmarshal(b, &w)
	if err != nil {
		return nil, "", fmt.Errorf("plist として読めません: %w", err)
	}
	switch f {
	case plist.BinaryFormat:
		return w, FormatBinary, nil
	case plist.XMLFormat:
		return w, FormatXML, nil
	}
	return w, Format(plist.FormatNames[f]), nil
}

// Unsign は署名済みショートカットから中の plist（Shortcut.wflow）を取り出す。
func Unsign(b []byte) ([]byte, error) {
	a, err := aea.Open(b)
	if err != nil {
		return nil, err
	}
	entries, err := aar.Parse(a.Payload)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Type == 'F' && e.Path == "Shortcut.wflow" {
			return e.Data, nil
		}
	}
	return nil, errors.New("アーカイブに Shortcut.wflow がありません")
}

// Encode はワークフローを指定形式で書き出す。
func Encode(w Workflow, f Format) ([]byte, error) {
	switch f {
	case FormatBinary:
		return plist.Marshal(map[string]any(w), plist.BinaryFormat)
	case FormatXML:
		return plist.MarshalIndent(map[string]any(w), plist.XMLFormat, "\t")
	case FormatJSON:
		return encodeJSON(w)
	case FormatSigned:
		return nil, errors.New("署名は macOS の `shortcuts sign` で行ってください")
	}
	return nil, fmt.Errorf("未知の形式 %q", f)
}

// Name はワークフロー名（あれば）を返す。
func (w Workflow) Name() string {
	s, _ := w["WFWorkflowName"].(string)
	return s
}

// Actions はアクションの一覧を返す。
func (w Workflow) Actions() []map[string]any {
	raw, _ := w["WFWorkflowActions"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, a := range raw {
		if m, ok := a.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
