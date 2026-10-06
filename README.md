# shortcut-tools

Apple ショートカットのワークフローを読む・変換する CLI（`sct`）です。
編集用のソース形式には [Cherri](https://github.com/electrikmilk/cherri) を使います。

## できること

| コマンド | 内容 |
| --- | --- |
| `sct show <入力>` | ワークフローを読みやすく表示（制御構文はインデント、変数参照は `{名前}`） |
| `sct convert -to xml\|binary\|json <入力>` | 形式を変換 |
| `sct decompile <入力>` | Cherri のコード（`.cherri`）に変換 |
| `sct compile <入力.cherri>` | Cherri のコードからショートカットを作成（macOS なら署名まで） |
| `sct defs <入力>...` | アプリ提供アクションの Cherri 定義を書き出す |

入力には次のどれでも渡せます（形式は自動判定）。

- 署名済み `.shortcut`（iOS 15 以降の共有ファイル、AEA 形式）
- バイナリ plist / XML plist（未署名の `.shortcut`、`.plist`）
- `sct convert -to json` で書き出した JSON

Cherri 自体は署名済みファイルを読めないので、`decompile` はいったん plist に展開してから Cherri に渡します。

## アプリ提供のアクション（Actions.app など）

Cherri が標準で知らないアクション（サードパーティのアプリや App Intents）は、
それを使っているショートカットから定義を作れます。

```sh
sct defs -o defs/actions-app.cherri "コメント コピー.shortcut"
```

```cherri
#include 'defs/actions-app.cherri'

globalVariableSetText("text_test", "hello")
const g = globalVariableGetText("text_test")
```

生成した定義には Cherri では書けない情報がコメントで入っています。

- `// @descriptor {...}`: アプリを特定する AppIntentDescriptor
- `// @entity key`: App Entity（`{title, subtitle, value}`）を受け取るパラメータ。Cherri では文字列で渡します

`sct compile` はこれを読んで、Cherri の出力に AppIntentDescriptor を足し、App Entity を包み直します。
よく使う定義は `defs/` に置いています。

## インストール

```sh
go install github.com/henoya/shortcut-tools/cmd/sct@latest
go install github.com/electrikmilk/cherri@latest   # decompile / compile を使う場合
```

`cherri` の場所は環境変数 `CHERRI` で変えられます。

## 注意

- **署名は macOS でしかできません。** `compile` は Cherri には常に `--skip-sign` を付け、補正したあと macOS なら `shortcuts sign` で署名します（`-no-sign` で省略）。ほかの OS では未署名ファイルだけを作ります。
- Cherri の `#include` は拡張子 `.cherri` まで書く必要があります。また、アクション定義は空行で区切らないと読み込まれません（`sct defs` の出力は区切っています）。
- XML plist は日付を秒単位でしか持てないため、XML に変換すると日付の秒未満が落ちます。JSON とバイナリは元の値を保ちます。
- JSON ではデータを `{"$data": "<base64>"}`、日付を `{"$date": "<RFC3339>"}` で表し、実数は `1.0` のように小数点付きで出力して整数と区別します。
- 署名済みファイルの展開は、ペイロードが 1 セグメントに収まるファイル（実質すべてのショートカット）を想定しています。

## 開発

```sh
go test ./...
```
