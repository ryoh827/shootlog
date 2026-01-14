# shootlog

写真のEXIFを取り込んで分析等行うためのツールにする予定です。

## 使い方

```bash
go run ./cmd/shootlog --input sample.jpg
```

標準出力に JSON 形式で EXIF 情報を出力します。
