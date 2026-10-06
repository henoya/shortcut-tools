package view

import (
	"strings"
	"testing"

	"github.com/henoya/shortcut-tools/internal/shortcut"
)

func act(id string, p map[string]any) any {
	return map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions." + id, "WFWorkflowActionParameters": p}
}

func TestRender(t *testing.T) {
	w := shortcut.Workflow{
		"WFWorkflowName": "サンプル",
		"WFWorkflowActions": []any{
			act("comment", map[string]any{"WFCommentActionText": "説明\n2 行目"}),
			act("gettext", map[string]any{"UUID": "U1", "WFTextActionText": "やあ"}),
			act("conditional", map[string]any{
				"GroupingIdentifier": "G", "WFControlFlowMode": int64(0), "WFCondition": int64(4),
				"WFInput": map[string]any{"Type": "Variable", "Variable": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value":               map[string]any{"Type": "ActionOutput", "OutputName": "テキスト", "OutputUUID": "U1"},
				}},
			}),
			act("alert", map[string]any{"WFAlertActionMessage": map[string]any{
				"WFSerializationType": "WFTextTokenString",
				"Value": map[string]any{
					"string":             "😀￼ です",
					"attachmentsByRange": map[string]any{"{2, 1}": map[string]any{"Type": "Variable", "VariableName": "名前"}},
				},
			}}),
			act("conditional", map[string]any{"GroupingIdentifier": "G", "WFControlFlowMode": int64(1)}),
			act("exit", map[string]any{}),
			act("conditional", map[string]any{"GroupingIdentifier": "G", "WFControlFlowMode": int64(2), "UUID": "U2", "CustomOutputName": "結果"}),
		},
	}
	var b strings.Builder
	if err := Render(&b, w, Options{}); err != nil {
		t.Fatal(err)
	}
	want := `# サンプル
アクション数: 7

   1 // 説明 ⏎ 2 行目
   2 gettext TextActionText="やあ" → {テキスト}
   3 conditional Condition=4 Input={テキスト}
   4   alert AlertActionMessage="😀{$名前} です"
   5 otherwise:
   6   exit
   7 end conditional → {結果}
`
	if b.String() != want {
		t.Errorf("表示が違います\ngot:\n%s\nwant:\n%s", b.String(), want)
	}
}

func TestRenderOtherwiseIf(t *testing.T) {
	input := map[string]any{"Type": "Variable", "Variable": map[string]any{
		"WFSerializationType": "WFTextTokenAttachment",
		"Value":               map[string]any{"Type": "Variable", "VariableName": "c"},
	}}
	w := shortcut.Workflow{"WFWorkflowActions": []any{
		act("conditional", map[string]any{"GroupingIdentifier": "G", "WFControlFlowMode": int64(0), "WFCondition": int64(4), "WFInput": input, "WFConditionalActionString": "a"}),
		act("alert", map[string]any{"WFAlertActionMessage": "1"}),
		act("conditional", map[string]any{"GroupingIdentifier": "G", "WFControlFlowMode": int64(1), "WFCondition": int64(4), "WFInput": input, "WFConditionalActionString": "b"}),
		act("alert", map[string]any{"WFAlertActionMessage": "2"}),
		act("conditional", map[string]any{"GroupingIdentifier": "G", "WFControlFlowMode": int64(1)}),
		act("alert", map[string]any{"WFAlertActionMessage": "3"}),
		act("conditional", map[string]any{"GroupingIdentifier": "G", "WFControlFlowMode": int64(2)}),
	}}
	var b strings.Builder
	if err := Render(&b, w, Options{}); err != nil {
		t.Fatal(err)
	}
	want := `   1 conditional Condition=4 ConditionalActionString="a" Input={$c}
   2   alert AlertActionMessage="1"
   3 otherwise if Condition=4 ConditionalActionString="b" Input={$c}:
   4   alert AlertActionMessage="2"
   5 otherwise:
   6   alert AlertActionMessage="3"
   7 end conditional
`
	if got := b.String(); !strings.HasSuffix(got, want) {
		t.Errorf("表示が違います\ngot:\n%s\nwant (末尾):\n%s", got, want)
	}
}
