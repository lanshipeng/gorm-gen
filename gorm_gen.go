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
	i18nCols    []string
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
	rootCmd.Flags().StringArrayVarP(&i18nCols, "i18n", "i", []string{}, "Specify cols to add i18n tag (e.g., <table>.<column>)")
	rootCmd.Flags().StringArrayVarP(&aliasModels, "alias", "a", []string{}, "Table alias mapping in form table=Model")

	// 标记 required 参数
	rootCmd.MarkFlagRequired("dsn")
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

	c, err := mysql2.ParseDSN(dsn)
	if err != nil {
		log.Fatal(err)
	}
	dbName := c.DBName
	dbName = strings.TrimPrefix(dbName, "dev_")
	dbName = strings.TrimPrefix(dbName, "test_")

	var (
		db *gorm.DB
	)

	db, err = gorm.Open(mysql.Open(dsn))
	if err != nil {
		log.Fatal(err)
	}

	db.NamingStrategy = NamingStrategy{}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	pkgPath := fmt.Sprintf("%smodel", strings.ToLower(dbName))
	if len(tables) == 0 {
		if err := db.Raw("SHOW TABLES").Scan(&tables).Error; err != nil {
			log.Printf("failed to get tables for db: %v", err)
			return
		}
	}

	outPath := filepath.Join(outDir, pkgPath)
	g := gen.NewGenerator(gen.Config{
		OutPath:           outPath,
		ModelPkgPath:      pkgPath,
		FieldNullable:     false,
		FieldWithIndexTag: true,
		FieldWithTypeTag:  true,
	})
	g.UseDB(db)
	g.WithOpts(gen.FieldIgnore("create_time", "update_time", "id"))
	g.WithOpts(gen.FieldJSONTagWithNS(func(columnName string) (tagContent string) {
		return strcase.ToLowerCamel(columnName)
	}))
	g.WithOpts(gen.FieldType("deleted", "bool"))

	i18nMap := parseI18nCols(i18nCols)
	aliasMap := parseAliasModels(aliasModels)

	for _, table := range tables {
		g.WithOpts(gen.FieldModify(func(field gen.Field) gen.Field {
			field.Name = strcase.ToCamel(field.ColumnName)

			if tableMap, ok := i18nMap[strings.ToLower(table)]; ok {
				if _, match := tableMap[strings.ToLower(field.ColumnName)]; match {
					if field.Tag != nil {
						field.Tag.Set("i18n", field.ColumnName)
					}
				}
			}
			return field
		}))

		if alias, ok := aliasMap[strings.ToLower(table)]; ok {
			g.GenerateModelAs(table, alias)
		} else {
			g.GenerateModel(table)
		}

		g.Execute()
	}

	log.Println("所有表结构的代码生成和处理完成！")
}

func parseI18nCols(cols []string) map[string]map[string]struct{} {
	m := make(map[string]map[string]struct{})
	for _, f := range cols {
		parts := strings.SplitN(f, ".", 2)
		if len(parts) != 2 {
			continue
		}
		table := strings.ToLower(parts[0])
		column := strings.ToLower(parts[1])
		if m[table] == nil {
			m[table] = make(map[string]struct{})
		}
		m[table][column] = struct{}{}
	}
	return m
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
