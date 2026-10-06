package defs

import (
	"reflect"
	"strings"
	"testing"

	"github.com/henoya/shortcut-tools/internal/shortcut"
)

var descriptor = map[string]any{
	"AppIntentIdentifier": "GlobalVariableGetText",
	"BundleIdentifier":    "com.example.App",
	"Name":                "App",
	"TeamIdentifier":      "ABCDE12345",
}

func workflow() shortcut.Workflow {
	return shortcut.Workflow{"WFWorkflowActions": []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "x"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "com.example.App.GlobalVariableGetText",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":                "U1",
				"AppIntentDescriptor": descriptor,
				"key": map[string]any{
					"title": map[string]any{"key": "k"}, "subtitle": map[string]any{"key": "k"}, "value": "k",
				},
				"WFCount": int64(3),
				"Mode":    map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{}},
			},
		},
	}}
}

func TestCollectAndWrite(t *testing.T) {
	acts := Collect(workflow())
	if len(acts) != 1 {
		t.Fatalf("標準アクションは除くはず: %d 件", len(acts))
	}
	var b strings.Builder
	if err := Write(&b, acts); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{
		`// @descriptor {"AppIntentIdentifier":"GlobalVariableGetText","BundleIdentifier":"com.example.App","Name":"App","TeamIdentifier":"ABCDE12345"}`,
		"// @entity key\n",
		"action 'com.example.App.GlobalVariableGetText' globalVariableGetText(variable mode: 'Mode', number count: 'WFCount', text key: 'key'): variable\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("出力に %q がありません:\n%s", want, got)
		}
	}
}

func TestRoundTripMeta(t *testing.T) {
	var b strings.Builder
	if err := Write(&b, Collect(workflow())); err != nil {
		t.Fatal(err)
	}
	metas, err := ParseMeta([]byte(b.String()))
	if err != nil {
		t.Fatal(err)
	}

	// Cherri が出すような、Descriptor が無く key が文字列のアクション
	w := shortcut.Workflow{"WFWorkflowActions": []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "com.example.App.GlobalVariableGetText",
			"WFWorkflowActionParameters": map[string]any{"key": "k"},
		},
	}}
	if n := Apply(w, metas); n != 1 {
		t.Fatalf("Apply: %d 件", n)
	}
	want := workflow().Actions()[1]["WFWorkflowActionParameters"].(map[string]any)
	got := w.Actions()[0]["WFWorkflowActionParameters"].(map[string]any)
	for _, k := range []string{"AppIntentDescriptor", "key"} {
		if !reflect.DeepEqual(got[k], want[k]) {
			t.Errorf("%s: got %#v, want %#v", k, got[k], want[k])
		}
	}
	if n := Apply(w, metas); n != 0 {
		t.Errorf("2 回目は何も変えないはず: %d 件", n)
	}
}
