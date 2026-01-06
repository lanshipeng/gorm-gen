package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/tools/imports"
	"gorm.io/gen"
	"gorm.io/gorm"
)

var (
	reStructDef     = regexp.MustCompile(`(?m)^type\s+(\w+)\s+struct\s*{`)
	reImportBlock   = regexp.MustCompile(`(?m)^import\s*\([^)]+\)`)
	rePackage       = regexp.MustCompile(`(?m)^package\s+\w+`)
	reTableNameFunc = regexp.MustCompile(`func\s+\(\*\s*(\w+)\s*\)\s+TableName\(\)\s+string\s*{`)
)

type CustomGenerator struct {
	*gen.Generator
	fieldsMap      map[string][]gen.Field
	ignoredColumns []string
}

func NewCustomGenerator(cfg gen.Config) *CustomGenerator {
	return &CustomGenerator{
		Generator:      gen.NewGenerator(cfg),
		fieldsMap:      make(map[string][]gen.Field),
		ignoredColumns: []string{},
	}
}

func (g *CustomGenerator) setTableFiled(table string, field gen.Field) {
	g.fieldsMap[table] = append(g.fieldsMap[table], field)
}

func (g *CustomGenerator) appendIgnoredColumns(columns string) {
	g.ignoredColumns = append(g.ignoredColumns, columns)
}

// ProcessFile 处理单个文件
func (g *CustomGenerator) processFile(table, filename string) error {
	content, err := os.ReadFile(filename) // #nosec G304
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	text := string(content)

	//text = addImportPkgPaths(text, "codeup.aliyun.com/shengshi/backend/toolbox/datax")
	//text = addBaseEntity(text)
	text = modifyTableName(text)
	//text = g.insertSerialVersion(text, table)

	text = formatCode(filename, text)

	// 写回文件
	if err := os.WriteFile(filename, []byte(text), 0600); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

func addImportPkgPaths(text string, importPkgPaths ...string) string {
	if len(importPkgPaths) == 0 {
		return text
	}

	add := ""
	for _, p := range importPkgPaths {
		add += fmt.Sprintf("\t\"%s\"\n", p)
	}

	// 如果已有 import 块
	if reImportBlock.MatchString(text) {
		return reImportBlock.ReplaceAllStringFunc(text, func(block string) string {
			return strings.TrimSuffix(block, ")") + add + ")"
		})
	}

	// 没有 import 块则插入新的
	importBlock := "import (\n" + add + ")\n\n"
	return rePackage.ReplaceAllStringFunc(text, func(pkg string) string {
		return pkg + "\n\n" + importBlock
	})
}

// formatCode 格式化 Go 源码
func formatCode(filename, src string) string {
	opt := &imports.Options{
		Comments:   true,
		TabIndent:  true,
		TabWidth:   8,
		FormatOnly: false,
	}
	imports.LocalPrefix = "codeup.aliyun.com"

	out, err := imports.Process(filename, []byte(src), opt)
	if err != nil {
		fmt.Println("imports.Process error:", err)
		return src
	}

	return string(out)
}

func addBaseEntity(text string) string {
	return reStructDef.ReplaceAllString(text, "type $1 struct {\n\tdatax.BaseEntity")
}

func modifyTableName(text string) string {
	return reTableNameFunc.ReplaceAllString(text, "func ($1) TableName() string {")
}

func (g *CustomGenerator) insertSerialVersion(text string, table string) string {
	fileds, ok := g.fieldsMap[table]
	if !ok {
		return text
	}

	serialVersion := getSerialVersion(fileds)
	text += fmt.Sprintf("\n// SerialVersion 序列号版本: %s\n", serialVersion)

	return text
}

func getSerialVersion(fields []gen.Field) string {
	// 初始化被忽略的字段
	fsigns := []string{"id.int64.id", "update_time.time.Time.updateTime", "create_time.time.Time.createTime"}

	for _, f := range fields {
		tag := f.Tag["json"]
		tag = strings.Split(tag, ",")[0]
		dbName := getGormColumn(f.Tag["gorm"])

		fsign := fmt.Sprintf("%s.%s.%s", dbName, f.Type, tag)
		fsigns = append(fsigns, fsign)
	}

	slices.Sort(fsigns)

	hash := md5.New()
	hash.Write([]byte(strings.Join(fsigns, ",")))
	h := hex.EncodeToString(hash.Sum(nil))
	return h[:6]
}

func getGormColumn(tag string) string {
	for part := range strings.SplitSeq(tag, ";") {
		if c, ok := strings.CutPrefix(part, "column:"); ok {
			return c
		}
	}
	return ""
}

type ColumnInfo struct {
	ColumnName    string `gorm:"column:COLUMN_NAME"`
	ColumnComment string `gorm:"column:COLUMN_COMMENT"`
}

// collectDeprecatedColumns
func (cg *CustomGenerator) collectDeprecatedColumns(db *gorm.DB, table string) {
	var cols []ColumnInfo

	err := db.Raw(`
		SELECT COLUMN_NAME, COLUMN_COMMENT
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
	`, table).Scan(&cols).Error
	if err != nil {
		log.Fatalf("failed to query columns for table %s: %v", table, err)
	}

	for _, col := range cols {
		if strings.Contains(strings.ToLower(col.ColumnComment), "@deprecated") {
			cg.appendIgnoredColumns(col.ColumnName)
		}
	}

	if len(cg.ignoredColumns) > 0 {
		log.Printf("ignored deprecated columns: %v", cg.ignoredColumns)
	}
}
