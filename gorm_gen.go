package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	mysql2 "github.com/go-sql-driver/mysql"
	"github.com/iancoleman/strcase"
	"github.com/spf13/cobra"
	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var (
	dsn         string
	tables      []string
	aliasModels []string
	outDir      string
	rootCmd     = &cobra.Command{
		Use:   "gorm-gen",
		Short: "A CLI tool to generate code from database schema",
		Run:   run,
	}
)

func init() {
	// 设置命令参数
	rootCmd.Flags().StringVarP(&dsn, "dsn", "d", "", "Database DSN (required)")
	rootCmd.Flags().StringArrayVarP(&tables, "table", "t", []string{}, "Table name(s), can be specified multiple times")
	rootCmd.Flags().StringVarP(&outDir, "out", "o", "./", "Output directory")
	rootCmd.Flags().StringArrayVarP(&aliasModels, "alias", "a", []string{}, "Table alias mapping in form table=Model")

	// 标记 required 参数
	_ = rootCmd.MarkFlagRequired("dsn")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	log.Println("DSN:", dsn)
	log.Println("Tables:", tables)
	log.Println("Output Dir:", outDir)

	var (
		db     *gorm.DB
		dbName string
	)

	c, err := mysql2.ParseDSN(dsn)
	if err != nil {
		log.Fatal(err)
	}
	dbName = c.DBName

	db, err = gorm.Open(mysql.Open(dsn))
	if err != nil {
		log.Fatal(err)
	}

	dbName = strings.TrimPrefix(dbName, "dev_")
	dbName = strings.TrimPrefix(dbName, "test_")

	db.NamingStrategy = NamingStrategy{}
	sqlDB, _ := db.DB()
	defer func() { _ = sqlDB.Close() }()

	pkgPath := fmt.Sprintf("%smodel", strings.ToLower(dbName))
	if len(tables) == 0 {
		if err := db.Raw("SHOW TABLES").Scan(&tables).Error; err != nil {
			log.Printf("failed to get tables for db: %v", err)
			return
		}
	}

	outPath := filepath.Join(outDir, pkgPath)

	if outDir != "./" {
		outPath = outDir
		pkgPath = filepath.Base(outDir)
	}

	cg := NewCustomGenerator(gen.Config{
		OutPath:           outPath,
		ModelPkgPath:      pkgPath,
		FieldNullable:     false,
		FieldWithIndexTag: true,
		FieldWithTypeTag:  true,
	})

	cg.UseDB(db)
	cg.WithOpts(gen.FieldJSONTagWithNS(func(columnName string) (tagContent string) {
		return strcase.ToLowerCamel(columnName)
	}))
	cg.WithOpts(gen.FieldType("deleted", "bool"))

	aliasMap := parseAliasModels(aliasModels)

	for _, table := range tables {
		cg.collectDeprecatedColumns(db, table)
		cg.WithOpts(gen.FieldIgnore(cg.ignoredColumns...))

		cg.WithOpts(gen.FieldModify(func(field gen.Field) gen.Field {
			cg.setTableFiled(table, field)

			field.Name = strcase.ToCamel(field.ColumnName)
			// 设置i18n tag
			if strings.Contains(strings.ToLower(field.ColumnComment), "@i18n") {
				if field.Tag != nil {
					field.Tag.Set("i18n", field.ColumnName)
				}
			}
			return field
		}))

		if alias, ok := aliasMap[strings.ToLower(table)]; ok {
			cg.GenerateModelAs(table, alias)
		} else {
			cg.GenerateModel(table)
		}
	}

	cg.Execute()

	for _, table := range tables {
		file := filepath.Join(outPath, fmt.Sprintf("%s.gen.go", table))
		cg.processFile(table, file)
	}

	log.Println("所有表结构的代码生成和处理完成！")
}

func parseAliasModels(aliasModels []string) map[string]string {
	m := make(map[string]string)
	for _, pair := range aliasModels {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			m[strings.ToLower(parts[0])] = parts[1]
		}
	}

	return m
}

type NamingStrategy struct {
	schema.NamingStrategy
}

func (NamingStrategy) SchemaName(table string) string {
	return strcase.ToCamel(table)
}
