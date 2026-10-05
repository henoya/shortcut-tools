package shortcut

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// JSON 表現では plist 固有の型を失わないよう、次のように包む。
//   データ: {"$data": "<base64>"}
//   日付:   {"$date": "<RFC3339>"}
//   実数:   小数点付きの数値（整数と区別するため 1 → 1.0）

func encodeJSON(w Workflow) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(toJSON(map[string]any(w))); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func toJSON(v any) any {
	switch v := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(v))
		for k, x := range v {
			m[k] = toJSON(x)
		}
		return m
	case []any:
		a := make([]any, len(v))
		for i, x := range v {
			a[i] = toJSON(x)
		}
		return a
	case []byte:
		return map[string]any{"$data": base64.StdEncoding.EncodeToString(v)}
	case time.Time:
		return map[string]any{"$date": v.UTC().Format(time.RFC3339Nano)}
	case float64:
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return strconv.FormatFloat(v, 'g', -1, 64)
		}
		s := strconv.FormatFloat(v, 'f', -1, 64)
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		return json.RawMessage(s)
	case float32:
		return toJSON(float64(v))
	}
	return v
}

func decodeJSON(b []byte) (Workflow, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var raw map[string]any
	if err := d.Decode(&raw); err != nil {
		return nil, fmt.Errorf("JSON として読めません: %w", err)
	}
	v, err := fromJSON(raw)
	if err != nil {
		return nil, err
	}
	return Workflow(v.(map[string]any)), nil
}

func fromJSON(v any) (any, error) {
	switch v := v.(type) {
	case map[string]any:
		if len(v) == 1 {
			if s, ok := v["$data"].(string); ok {
				return base64.StdEncoding.DecodeString(s)
			}
			if s, ok := v["$date"].(string); ok {
				return time.Parse(time.RFC3339Nano, s)
			}
		}
		m := make(map[string]any, len(v))
		for k, x := range v {
			y, err := fromJSON(x)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			m[k] = y
		}
		return m, nil
	case []any:
		a := make([]any, len(v))
		for i, x := range v {
			y, err := fromJSON(x)
			if err != nil {
				return nil, err
			}
			a[i] = y
		}
		return a, nil
	case json.Number:
		s := v.String()
		if strings.ContainsAny(s, ".eE") {
			return v.Float64()
		}
		if strings.HasPrefix(s, "-") {
			return v.Int64()
		}
		return strconv.ParseUint(s, 10, 64)
	}
	return v, nil
}
