# dbmodlegen

🚀 一个基于gorm数据库代码生成工具，自动生成model层代码。

## ✨ 功能特性

- 支持通过 DSN 连接数据库
- 支持通过 `-t` 参数指定多个表名
- 支持自定义输出目录(默认为当前目录)

## 📦 安装

```bash
go install github.com/lanshipeng/gorm-gen@latest
```

## 🔧 使用方式

- 单表生成
```bash
gorm-gen -d "mysql://root:123456@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local" -t tag -o "yourpath"
```
- 多表生成
```bash
gorm-gen -d "mysql://root:123456@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local" -t tag -t ban_rules -o "yourpath"
```

- db下所有表生成
```bash
gorm-gen -d "mysql://root:123456@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local" -o "yourpath"
```

- 指定表中字段生成i18n tag
```bash
gorm-gen -d "mysql://root:123456@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local" -t tag -t ban_rules -o "yourpath" -i tag.name -i ban_rules.name
```

- 指定生成的结构体名称
```bash
gorm-gen -d "mysql://root:123456@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local" -t tag -t ban_rules -o "yourpath" -a tag=OpTag -a ban_rules=OpBanRules
```

## 🧱 命令结构

```bash
gorm-gen --help

A CLI tool to generate code from database schema

Usage:
  gorm-gen [flags]

Flags:
  -a, --alias stringArray   Table alias mapping in form table=Model
  -d, --dsn string          Database DSN (required)
  -h, --help                help for gorm-gen
  -i, --i18n stringArray    Specify cols to add i18n tag (e.g., <table>.<column>)
  -o, --out string          Output directory (default "./")
  -t, --table stringArray   Table name(s), can be specified multiple times
```
