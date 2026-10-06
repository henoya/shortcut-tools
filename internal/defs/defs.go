// Package defs はショートカットに含まれるアプリ提供アクション（サードパーティや
// App Intents）から、Cherri のアクション定義を作る。
package defs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	"github.com/henoya/shortcut-tools/internal/shortcut"
)

// Action は 1 つのアクションの定義。
type Action struct {
	Identifier string
	Params     map[string]string // パラメータのキー → Cherri の型
	Order      []string          // 初めて見た順のキー
	HasOutput  bool
	// Descriptor は元の AppIntentDescriptor（あれば）。Cherri は出力しないので記録だけ残す。
	Descriptor map[string]any
	// Entities は App Entity を受け取るパラメータのキー。
	Entities []string
}

// 定義に含めないパラメータ。
var skipParams = map[string]bool{
	"UUID": true, "CustomOutputName": true, "AppIntentDescriptor": true,
	"GroupingIdentifier": true, "WFControlFlowMode": true,
}

// IsBuiltin はショートカット標準のアクションかどうかを返す。
// is.workflow.actions.* と com.apple.* は Cherri の標準定義に任せる。
func IsBuiltin(id string) bool {
	return strings.HasPrefix(id, "is.workflow.actions.") || strings.HasPrefix(id, "com.apple.")
}

// Collect はワークフロー群からアプリ提供アクションの定義を集める。
// 同じアクションが何度も出てきたら、パラメータを合わせて 1 つにする。
func Collect(ws ...shortcut.Workflow) []*Action {
	byID := map[string]*Action{}
	for _, w := range ws {
		for _, a := range w.Actions() {
			id, _ := a["WFWorkflowActionIdentifier"].(string)
			if id == "" || IsBuiltin(id) {
				continue
			}
			def := byID[id]
			if def == nil {
				def = &Action{Identifier: id, Params: map[string]string{}}
				byID[id] = def
			}
			p, _ := a["WFWorkflowActionParameters"].(map[string]any)
			if d, ok := p["AppIntentDescriptor"].(map[string]any); ok && def.Descriptor == nil {
				def.Descriptor = d
			}
			if _, ok := p["UUID"]; ok {
				def.HasOutput = true
			}
			keys := make([]string, 0, len(p))
			for k := range p {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if skipParams[k] {
					continue
				}
				t := typeOf(p[k])
				if isEntity(p[k]) {
					t = "text"
					if !contains(def.Entities, k) {
						def.Entities = append(def.Entities, k)
					}
				}
				switch old, seen := def.Params[k]; {
				case !seen:
					def.Params[k] = t
					def.Order = append(def.Order, k)
				case old != t && t != "variable":
					// 値の種類が場所によって違うときは何でも受け取れる型にする。
					if old != "variable" {
						def.Params[k] = "variable"
					}
				}
			}
		}
	}
	out := make([]*Action, 0, len(byID))
	for _, d := range byID {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Identifier < out[j].Identifier })
	return out
}

// isEntity は App Entity の値（{title, subtitle, value}）かどうかを返す。
// Cherri では書けないので、定義では文字列として受け取り、sct compile で包み直す。
func isEntity(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	_, hasValue := m["value"]
	_, hasTitle := m["title"]
	_, ser := m["WFSerializationType"]
	return hasValue && hasTitle && !ser
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// typeOf はパラメータの値から Cherri の型を推測する。
func typeOf(v any) string {
	switch v := v.(type) {
	case string:
		return "text"
	case bool:
		return "bool"
	case int64, uint64:
		return "number"
	case float64:
		return "float"
	case []any:
		return "array"
	case map[string]any:
		switch v["WFSerializationType"] {
		case "WFTextTokenString":
			return "text"
		case "WFDictionaryFieldValue":
			return "dictionary"
		case "WFNumberSubstitutableState":
			return "number"
		}
		// 変数参照や App Entity など、その他の構造は変数として受け取る。
		return "variable"
	}
	return "variable"
}

// Write は Cherri のアクション定義として書き出す。
func Write(out io.Writer, actions []*Action) error {
	var b strings.Builder
	b.WriteString("// sct defs で生成したアプリ提供アクションの定義\n")
	for _, a := range actions {
		b.WriteString("\n")
		if d := a.Descriptor; d != nil {
			fmt.Fprintf(&b, "// %v (%v) / AppIntent: %v\n", d["Name"], d["BundleIdentifier"], d["AppIntentIdentifier"])
			j, err := json.Marshal(d)
			if err != nil {
				return err
			}
			fmt.Fprintf(&b, "%s%s\n", descriptorMarker, j)
		}
		if len(a.Entities) > 0 {
			fmt.Fprintf(&b, "%s%s\n", entityMarker, strings.Join(a.Entities, " "))
		}
		params := make([]string, 0, len(a.Order))
		used := map[string]bool{}
		for _, k := range a.Order {
			name := uniqueName(paramName(k), used)
			params = append(params, fmt.Sprintf("%s %s: '%s'", a.Params[k], name, k))
		}
		fmt.Fprintf(&b, "action '%s' %s(%s)", a.Identifier, funcName(a.Identifier), strings.Join(params, ", "))
		if a.HasOutput {
			b.WriteString(": variable")
		}
		b.WriteString("\n")
	}
	_, err := io.WriteString(out, b.String())
	return err
}

// funcName は識別子の最後の部分を lowerCamelCase の関数名にする。
func funcName(id string) string {
	parts := strings.Split(id, ".")
	return lowerCamel(parts[len(parts)-1])
}

func paramName(key string) string {
	return lowerCamel(strings.TrimPrefix(key, "WF"))
}

// lowerCamel は英数字以外を区切りとして lowerCamelCase にする。
func lowerCamel(s string) string {
	var b strings.Builder
	upperNext := false
	for _, r := range s {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			upperNext = b.Len() > 0
			continue
		}
		switch {
		case b.Len() == 0:
			b.WriteRune(unicode.ToLower(r))
		case upperNext:
			b.WriteRune(unicode.ToUpper(r))
		default:
			b.WriteRune(r)
		}
		upperNext = false
	}
	name := b.String()
	if name == "" || unicode.IsDigit(rune(name[0])) {
		name = "p" + name
	}
	return name
}

func uniqueName(name string, used map[string]bool) string {
	n := name
	for i := 2; used[n]; i++ {
		n = fmt.Sprintf("%s%d", name, i)
	}
	used[n] = true
	return n
}

// descriptorMarker の行には AppIntentDescriptor を JSON で書く。
// Cherri はこれを出力しないので、sct compile が後から補う。
const descriptorMarker = "// @descriptor "

// entityMarker の行には App Entity を受け取るパラメータのキーを空白区切りで書く。
const entityMarker = "// @entity "

// Meta は Cherri では表せず、sct compile が補う情報。
type Meta struct {
	Descriptor map[string]any
	Entities   []string
}

// ParseMeta は Cherri のソースから @descriptor / @entity 行を読み、
// 直後のアクション定義の識別子と対応付ける。
func ParseMeta(src []byte) (map[string]Meta, error) {
	out := map[string]Meta{}
	var pending Meta
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, descriptorMarker):
			var d map[string]any
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, descriptorMarker)), &d); err != nil {
				return nil, fmt.Errorf("@descriptor を読めません: %w", err)
			}
			pending.Descriptor = d
		case strings.HasPrefix(line, entityMarker):
			pending.Entities = strings.Fields(strings.TrimPrefix(line, entityMarker))
		case strings.HasPrefix(line, "action "):
			if pending.Descriptor != nil || len(pending.Entities) > 0 {
				if id := actionIdentifier(line); id != "" {
					out[id] = pending
				}
			}
			pending = Meta{}
		}
	}
	return out, sc.Err()
}

// actionIdentifier は「action [修飾子...] 'identifier' name(...)」から識別子を取り出す。
func actionIdentifier(line string) string {
	i := strings.IndexByte(line, '\'')
	if i < 0 {
		return ""
	}
	j := strings.IndexByte(line[i+1:], '\'')
	if j < 0 {
		return ""
	}
	return line[i+1 : i+1+j]
}

// Apply は Meta の情報をワークフローに補い、変更したアクションの数を返す。
//   - AppIntentDescriptor が無ければ付ける
//   - App Entity のパラメータに文字列が入っていれば {title, subtitle, value} に包む
func Apply(w shortcut.Workflow, metas map[string]Meta) int {
	n := 0
	for _, a := range w.Actions() {
		id, _ := a["WFWorkflowActionIdentifier"].(string)
		m, ok := metas[id]
		if !ok {
			continue
		}
		p, _ := a["WFWorkflowActionParameters"].(map[string]any)
		if p == nil {
			p = map[string]any{}
			a["WFWorkflowActionParameters"] = p
		}
		changed := false
		if _, has := p["AppIntentDescriptor"]; !has && m.Descriptor != nil {
			p["AppIntentDescriptor"] = m.Descriptor
			changed = true
		}
		for _, k := range m.Entities {
			if s, ok := p[k].(string); ok {
				p[k] = map[string]any{
					"title":    map[string]any{"key": s},
					"subtitle": map[string]any{"key": s},
					"value":    s,
				}
				changed = true
			}
		}
		if changed {
			n++
		}
	}
	return n
}
