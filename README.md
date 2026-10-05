# shortcut-tools

Apple ショートカットのワークフローを読む・変換する CLI（`sct`）です。
編集用のソース形式には [Cherri](https://github.com/electrikmilk/cherri) を使います。

## できること

| コマンド | 内容 |
| --- | --- |
| `sct show <入力>` | ワークフローを読みやすく表示（制御構文はインデント、変数参照は `{名前}`） |
| `sct convert -to xml\|binary\|json <入力>` | 形式を変換 |
| `sct decompile <入力>` | Cherri のコード（`.cherri`）に変換 |
| `sct compile <入力.cherri>` | Cherri のコードからショートカットを作成 |

入力には次のどれでも渡せます（形式は自動判定）。

- 署名済み `.shortcut`（iOS 15 以降の共有ファイル、AEA 形式）
- バイナリ plist / XML plist（未署名の `.shortcut`、`.plist`）
- `sct convert -to json` で書き出した JSON

Cherri 自体は署名済みファイルを読めないので、`decompile` はいったん plist に展開してから Cherri に渡します。

## インストール

```sh
go install github.com/henoya/shortcut-tools/cmd/sct@latest
go install github.com/electrikmilk/cherri@latest   # decompile / compile を使う場合
```

`cherri` の場所は環境変数 `CHERRI` で変えられます。

## 注意

- **署名は macOS でしかできません。** `compile` は macOS 以外では `--skip-sign` を付けて未署名ファイルを作ります。署名は Mac で `shortcuts sign` を実行してください。
- XML plist は日付を秒単位でしか持てないため、XML に変換すると日付の秒未満が落ちます。JSON とバイナリは元の値を保ちます。
- JSON ではデータを `{"$data": "<base64>"}`、日付を `{"$date": "<RFC3339>"}` で表し、実数は `1.0` のように小数点付きで出力して整数と区別します。
- 署名済みファイルの展開は、ペイロードが 1 セグメントに収まるファイル（実質すべてのショートカット）を想定しています。

## 開発

```sh
go test ./...
```
