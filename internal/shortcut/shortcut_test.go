package shortcut

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
	"time"

	"howett.net/plist"
)

func sample() Workflow {
	return Workflow{
		"WFWorkflowName":                 "テスト",
		"WFWorkflowClientVersion":        "2607.0.2",
		"WFWorkflowMinimumClientVersion": uint64(900),
		"WFWorkflowIcon": map[string]any{
			"WFWorkflowIconStartColor":  uint64(4282601983),
			"WFWorkflowIconGlyphNumber": uint64(59511),
		},
		"WFWorkflowActions": []any{
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
				"WFWorkflowActionParameters": map[string]any{
					"UUID":             "A1",
					"WFTextActionText": "こんにちは",
				},
			},
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.number",
				"WFWorkflowActionParameters": map[string]any{
					"WFNumberActionNumber": 1.0,
					"Negative":             int64(-3),
					"Blob":                 []byte{0, 1, 2},
					"When":                 time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC),
					"Flag":                 true,
				},
			},
		},
	}
}

func TestRoundTrip(t *testing.T) {
	want := sample()
	for _, f := range []Format{FormatBinary, FormatXML, FormatJSON} {
		b, err := Encode(want, f)
		if err != nil {
			t.Fatalf("%s: Encode: %v", f, err)
		}
		got, gf, err := Decode(b)
		if err != nil {
			t.Fatalf("%s: Decode: %v", f, err)
		}
		if gf != f {
			t.Errorf("形式の判定: got %s, want %s", gf, f)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: 往復で内容が変わりました\ngot  %#v\nwant %#v", f, got, want)
		}
	}
}

func TestJSONKeepsRealType(t *testing.T) {
	b, err := Encode(sample(), FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"WFNumberActionNumber": 1.0`)) {
		t.Errorf("実数が整数と区別できる形で出力されていません:\n%s", b)
	}
}

// fakeSigned は署名済みショートカットと同じ配置のバイト列を作る。
// 署名部分はダミーで、ペイロードは LZFSE の非圧縮ブロックに入れる。
func fakeSigned(t *testing.T, wflow []byte) []byte {
	t.Helper()
	var aa bytes.Buffer
	writeEntry := func(fields []byte, blob []byte) {
		aa.WriteString("AA01")
		binary.Write(&aa, binary.LittleEndian, uint16(6+len(fields)))
		aa.Write(fields)
		aa.Write(blob)
	}
	var dir bytes.Buffer
	dir.WriteString("TYP1D")
	dir.WriteString("PATP\x00\x00")
	writeEntry(dir.Bytes(), nil)

	var file bytes.Buffer
	file.WriteString("TYP1F")
	file.WriteString("PATP")
	binary.Write(&file, binary.LittleEndian, uint16(len("Shortcut.wflow")))
	file.WriteString("Shortcut.wflow")
	file.WriteString("MOD2\xa4\x01")
	file.WriteString("MTMT" + string(make([]byte, 12)))
	file.WriteString("DATB")
	binary.Write(&file, binary.LittleEndian, uint32(len(wflow)))
	writeEntry(file.Bytes(), wflow)

	auth, err := plist.Marshal(map[string]any{"dummy": true}, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	out.WriteString("AEA1")
	out.Write([]byte{0, 0, 0, 0})
	binary.Write(&out, binary.LittleEndian, uint32(len(auth)))
	out.Write(auth)
	out.Write(make([]byte, 0x495c-0xc))
	out.WriteString("bvx-")
	binary.Write(&out, binary.LittleEndian, uint32(aa.Len()))
	out.Write(aa.Bytes())
	out.WriteString("bvx$")
	return out.Bytes()
}

func TestDecodeSigned(t *testing.T) {
	want := sample()
	wflow, err := Encode(want, FormatBinary)
	if err != nil {
		t.Fatal(err)
	}
	got, f, err := Decode(fakeSigned(t, wflow))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f != FormatSigned {
		t.Errorf("形式の判定: got %s", f)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("署名済みから取り出した内容が違います")
	}
}
