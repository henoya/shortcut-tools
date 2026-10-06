// Package view はワークフローを人が読みやすいテキストにする。
package view

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/henoya/shortcut-tools/internal/shortcut"
)

const actionPrefix = "is.workflow.actions."

// Options は表示の設定。
type Options struct {
	// MaxValue は 1 つの値を表示する最大文字数（0 で無制限）。
	MaxValue int
}

// Render はワークフローの概要とアクション一覧を書き出す。
func Render(out io.Writer, w shortcut.Workflow, opt Options) error {
	r := &renderer{out: out, opt: opt, outputs: outputNames(w)}
	r.header(w)
	for i, a := range w.Actions() {
		r.action(i+1, a)
	}
	return r.err
}

type renderer struct {
	out     io.Writer
	opt     Options
	indent  int
	outputs map[string]string // UUID → 出力名
	err     error
}

func (r *renderer) printf(format string, args ...any) {
	if r.err == nil {
		_, r.err = fmt.Fprintf(r.out, format, args...)
	}
}

func (r *renderer) header(w shortcut.Workflow) {
	name := w.Name()
	if name == "" {
		name = "(名前なし)"
	}
	r.printf("# %s\n", name)
	r.printf("アクション数: %d\n", len(w.Actions()))
	if v, ok := w["WFWorkflowMinimumClientVersionString"].(string); ok {
		r.printf("最低クライアントバージョン: %s\n", v)
	}
	if ts := strList(w["WFWorkflowTypes"]); len(ts) > 0 {
		r.printf("種類: %s\n", strings.Join(ts, ", "))
	}
	if cs := strList(w["WFWorkflowInputContentItemClasses"]); len(cs) > 0 {
		for i, c := range cs {
			cs[i] = strings.TrimSuffix(strings.TrimPrefix(c, "WF"), "ContentItem")
		}
		r.printf("受け付ける入力: %s\n", strings.Join(cs, ", "))
	}
	if qs, ok := w["WFWorkflowImportQuestions"].([]any); ok && len(qs) > 0 {
		r.printf("読み込み時の質問: %d 件\n", len(qs))
	}
	r.printf("\n")
}

func (r *renderer) action(n int, a map[string]any) {
	id, _ := a["WFWorkflowActionIdentifier"].(string)
	p, _ := a["WFWorkflowActionParameters"].(map[string]any)
	short := strings.TrimPrefix(id, actionPrefix)

	mode, hasMode := intVal(p["WFControlFlowMode"])
	if hasMode && mode > 0 && r.indent > 0 {
		r.indent--
	}
	pad := strings.Repeat("  ", r.indent)

	switch {
	case short == "comment":
		text, _ := p["WFCommentActionText"].(string)
		r.printf("%4d %s// %s\n", n, pad, r.clip(oneLine(text)))
		return
	case hasMode && mode == 1:
		label := "otherwise"
		switch {
		case p["WFMenuItemTitle"] != nil:
			label = "case " + r.value(p["WFMenuItemTitle"])
		case short == "conditional" && hasCondition(p):
			// iOS 27 の「Otherwise If」は、条件付きの途中アクションとして保存される。
			label = "otherwise if" + r.params(p)
		}
		r.printf("%4d %s%s:\n", n, pad, label)
		r.indent++
		return
	case hasMode && mode == 2:
		r.printf("%4d %send %s%s\n", n, pad, short, r.outputSuffix(p))
		return
	}

	r.printf("%4d %s%s%s%s\n", n, pad, short, r.params(p), r.outputSuffix(p))
	if hasMode && mode == 0 {
		r.indent++
	}
}

// hasCondition は条件分岐のパラメータに条件が含まれるかどうかを返す。
func hasCondition(p map[string]any) bool {
	for _, k := range []string{"WFCondition", "WFConditions", "WFInput"} {
		if _, ok := p[k]; ok {
			return true
		}
	}
	return false
}

// 表示しないパラメータ（制御用・識別用）。
var hidden = map[string]bool{
	"UUID": true, "GroupingIdentifier": true, "WFControlFlowMode": true, "CustomOutputName": true,
}

func (r *renderer) params(p map[string]any) string {
	keys := make([]string, 0, len(p))
	for k := range p {
		if !hidden[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(" ")
		b.WriteString(strings.TrimPrefix(k, "WF"))
		b.WriteString("=")
		b.WriteString(r.value(p[k]))
	}
	return b.String()
}

func (r *renderer) outputSuffix(p map[string]any) string {
	if name, ok := p["CustomOutputName"].(string); ok {
		return " → {" + name + "}"
	}
	if id, ok := p["UUID"].(string); ok {
		if name, ok := r.outputs[id]; ok {
			return " → {" + name + "}"
		}
	}
	return ""
}

// value はパラメータの値を短い文字列にする。
func (r *renderer) value(v any) string {
	switch v := v.(type) {
	case string:
		return strconv.Quote(r.clip(v))
	case bool, int64, uint64, float64:
		return fmt.Sprint(v)
	case []any:
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = r.value(x)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		return r.object(v)
	case []byte:
		return fmt.Sprintf("<データ %d バイト>", len(v))
	}
	return r.clip(fmt.Sprint(v))
}

func (r *renderer) object(m map[string]any) string {
	ser, _ := m["WFSerializationType"].(string)
	val := m["Value"]
	switch ser {
	case "WFTextTokenAttachment":
		if vm, ok := val.(map[string]any); ok {
			return attachment(vm)
		}
	case "WFTextTokenString":
		if vm, ok := val.(map[string]any); ok {
			return strconv.Quote(r.clip(tokenString(vm)))
		}
	case "WFNumberSubstitutableState", "WFStringSubstitutableState":
		return r.value(val)
	case "WFDictionaryFieldValue":
		if vm, ok := val.(map[string]any); ok {
			// 入れ子の辞書は {Value, WFSerializationType} でもう一段包まれている。
			if _, wrapped := vm["WFSerializationType"]; wrapped {
				return r.object(vm)
			}
			return r.dictionary(vm)
		}
	}
	// 条件の入力などで使われる {"Type":"Variable","Variable":{...}} 形式。
	if t, _ := m["Type"].(string); t == "Variable" {
		if inner, ok := m["Variable"].(map[string]any); ok {
			return r.value(inner)
		}
	}
	b, _ := json.Marshal(m)
	return r.clip(string(b))
}

func (r *renderer) dictionary(v map[string]any) string {
	items, _ := v["WFDictionaryFieldValueItems"].([]any)
	parts := make([]string, 0, len(items))
	for _, it := range items {
		im, _ := it.(map[string]any)
		parts = append(parts, r.value(im["WFKey"])+": "+r.value(im["WFValue"]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// attachment は変数参照を {名前} の形にする。
func attachment(v map[string]any) string {
	t, _ := v["Type"].(string)
	var name string
	switch t {
	case "ActionOutput":
		name, _ = v["OutputName"].(string)
	case "Variable":
		vn, _ := v["VariableName"].(string)
		name = "$" + vn
	case "ExtensionInput":
		name = "ショートカットの入力"
	case "CurrentDate":
		name = "現在の日付"
	case "Clipboard":
		name = "クリップボード"
	case "Ask":
		name = "毎回尋ねる"
	case "DeviceDetails":
		name = "デバイスの詳細"
	case "Input":
		name = "入力"
	default:
		name = t
	}
	if ag, ok := v["Aggrandizements"].([]any); ok {
		for _, a := range ag {
			am, _ := a.(map[string]any)
			if p, ok := am["PropertyName"].(string); ok {
				name += "." + p
			} else if k, ok := am["DictionaryKey"].(string); ok {
				name += "[" + strconv.Quote(k) + "]"
			} else if c, ok := am["CoercionItemClass"].(string); ok {
				name += " as " + strings.TrimSuffix(strings.TrimPrefix(c, "WF"), "ContentItem")
			}
		}
	}
	return "{" + name + "}"
}

var rangeKey = regexp.MustCompile(`^\{(\d+),\s*(\d+)\}$`)

// tokenString は U+FFFC で埋め込まれた変数を {名前} に置き換える。
// 位置は UTF-16 単位で数えられている。
func tokenString(v map[string]any) string {
	s, _ := v["string"].(string)
	atts, _ := v["attachmentsByRange"].(map[string]any)
	if len(atts) == 0 {
		return s
	}
	type rep struct {
		pos, n int
		text   string
	}
	var reps []rep
	for k, a := range atts {
		m := rangeKey.FindStringSubmatch(k)
		am, ok := a.(map[string]any)
		if m == nil || !ok {
			continue
		}
		pos, _ := strconv.Atoi(m[1])
		n, _ := strconv.Atoi(m[2])
		reps = append(reps, rep{pos, n, attachment(am)})
	}
	sort.Slice(reps, func(i, j int) bool { return reps[i].pos > reps[j].pos })
	u := utf16.Encode([]rune(s))
	for _, rp := range reps {
		if rp.pos < 0 || rp.pos+rp.n > len(u) {
			continue
		}
		mid := utf16.Encode([]rune(rp.text))
		u = append(u[:rp.pos:rp.pos], append(mid, u[rp.pos+rp.n:]...)...)
	}
	return string(utf16.Decode(u))
}

// outputNames は参照されているアクション出力の UUID と名前の対応を集める。
func outputNames(w shortcut.Workflow) map[string]string {
	names := map[string]string{}
	var walk func(v any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if id, ok := v["OutputUUID"].(string); ok {
				if n, ok := v["OutputName"].(string); ok {
					names[id] = n
				}
			}
			for _, x := range v {
				walk(x)
			}
		case []any:
			for _, x := range v {
				walk(x)
			}
		}
	}
	walk(map[string]any(w))
	return names
}

func (r *renderer) clip(s string) string {
	if r.opt.MaxValue <= 0 {
		return s
	}
	rs := []rune(s)
	if len(rs) <= r.opt.MaxValue {
		return s
	}
	return string(rs[:r.opt.MaxValue]) + "…"
}

func oneLine(s string) string { return strings.ReplaceAll(s, "\n", " ⏎ ") }

func strList(v any) []string {
	a, _ := v.([]any)
	out := make([]string, 0, len(a))
	for _, x := range a {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func intVal(v any) (int, bool) {
	switch v := v.(type) {
	case int64:
		return int(v), true
	case uint64:
		return int(v), true
	case float64:
		return int(v), true
	}
	return 0, false
}
