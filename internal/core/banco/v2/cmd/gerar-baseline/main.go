package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql/schema"
	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"

	"go-concierge/internal/core/banco/v2/ent/annotation"
)

const schemaPath = "./internal/core/banco/v2/ent/schema"
const baselinePath = "internal/core/banco/v2/migrations/00001_baseline.sql"

type reference struct {
	owner string
	annotation.ForeignKey
}

func main() {
	check := flag.Bool("check", false, "confere a baseline versionada sem escrever arquivos")
	flag.Parse()
	if flag.NArg() != 0 {
		log.Fatal("uso: go run ./internal/core/banco/v2/cmd/gerar-baseline [-check], na raiz do repositório")
	}
	if err := run(context.Background(), *check); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, check bool) error {
	graph, err := entc.LoadGraph(schemaPath, &gen.Config{})
	if err != nil {
		return fmt.Errorf("carregar schemas Ent v2: %w", err)
	}
	tables, err := graph.Tables()
	if err != nil {
		return fmt.Errorf("construir tabelas Ent v2: %w", err)
	}
	sort.Slice(tables, func(i, j int) bool { return tables[i].Name < tables[j].Name })
	byName := make(map[string]*schema.Table, len(tables))
	for _, table := range tables {
		byName[table.Name] = table
	}
	var refs []reference
	var nullsNotDistinct []string
	for _, node := range graph.Nodes {
		raw, ok := node.Annotations[(annotation.Constraints{}).Name()]
		if !ok {
			continue
		}
		encoded, err := json.Marshal(raw)
		if err != nil {
			return fmt.Errorf("codificar constraints de %s: %w", node.Table(), err)
		}
		var constraints annotation.Constraints
		if err := json.Unmarshal(encoded, &constraints); err != nil {
			return fmt.Errorf("ler constraints de %s: %w", node.Table(), err)
		}
		owner := byName[node.Table()]
		for _, name := range constraints.NullsNotDistinct {
			if !slices.ContainsFunc(owner.Indexes, func(index *schema.Index) bool { return index.Name == name && index.Unique }) {
				return fmt.Errorf("índice único %s não encontrado em %s", name, owner.Name)
			}
			nullsNotDistinct = append(nullsNotDistinct, name)
		}
		for _, fk := range constraints.ForeignKeys {
			target := byName[fk.Table]
			if target == nil || len(fk.Columns) == 0 || len(fk.Columns) != len(fk.Target) {
				return fmt.Errorf("referência inválida em %s.%s", owner.Name, fk.Symbol)
			}
			columns, err := lookupColumns(owner, fk.Columns)
			if err != nil {
				return err
			}
			targetColumns, err := lookupColumns(target, fk.Target)
			if err != nil {
				return err
			}
			for _, name := range fk.SetNullColumns {
				column, ok := owner.Column(name)
				if !ok || !column.Nullable || !slices.Contains(fk.Columns, name) {
					return fmt.Errorf("coluna SET NULL inválida em %s.%s: %s", owner.Name, fk.Symbol, name)
				}
			}
			refs = append(refs, reference{owner: owner.Name, ForeignKey: fk})
			if len(fk.SetNullColumns) == 0 {
				owner.AddForeignKey(&schema.ForeignKey{
					Symbol: fk.Symbol, Columns: columns, RefTable: target, RefColumns: targetColumns,
					OnUpdate: schema.NoAction, OnDelete: schema.NoAction,
				})
			}
		}
	}
	slices.Sort(nullsNotDistinct)
	sort.Slice(refs, func(i, j int) bool { return refs[i].Symbol < refs[j].Symbol })
	ddl, err := schema.DDL(ctx, schema.DDLArgs{Dialect: dialect.Postgres, Version: "18.0.0", Tables: tables})
	if err != nil {
		return fmt.Errorf("planejar DDL PostgreSQL 18: %w", err)
	}
	var lines []string
	for _, line := range strings.Split(ddl, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			lines = append(lines, line)
		}
	}
	ddl = strings.TrimSpace(strings.Join(lines, "\n"))
	for _, name := range nullsNotDistinct {
		pattern := regexp.MustCompile(`(?m)^CREATE UNIQUE INDEX ` + regexp.QuoteMeta(quote(name)) + `[^\n]*;$`)
		matches := pattern.FindAllStringIndex(ddl, -1)
		if len(matches) != 1 {
			return fmt.Errorf("DDL do índice %s: esperado um comando, encontrados %d", name, len(matches))
		}
		ddl = pattern.ReplaceAllStringFunc(ddl, func(command string) string {
			return strings.TrimSuffix(command, ";") + " NULLS NOT DISTINCT;"
		})
	}
	var result strings.Builder
	result.WriteString("-- +goose Up\n")
	result.WriteString(ddl)
	result.WriteString("\n")
	for _, ref := range refs {
		if len(ref.SetNullColumns) > 0 {
			fmt.Fprintf(&result, "ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s) ON UPDATE NO ACTION ON DELETE SET NULL (%s);\n",
				quote(ref.owner), quote(ref.Symbol), quoteList(ref.Columns), quote(ref.Table), quoteList(ref.Target), quoteList(ref.SetNullColumns))
		}
	}
	result.WriteString("\n-- +goose Down\n")
	for _, ref := range refs {
		fmt.Fprintf(&result, "ALTER TABLE %s DROP CONSTRAINT %s;\n", quote(ref.owner), quote(ref.Symbol))
	}
	for i := len(tables) - 1; i >= 0; i-- {
		fmt.Fprintf(&result, "DROP TABLE %s;\n", quote(tables[i].Name))
	}
	generated := []byte(result.String())
	if check {
		current, err := os.ReadFile(baselinePath)
		if err != nil {
			return fmt.Errorf("ler baseline versionada: %w", err)
		}
		if !bytes.Equal(current, generated) {
			return errors.New("baseline v2 diverge dos schemas; execute go run ./internal/core/banco/v2/cmd/gerar-baseline e revise o diff")
		}
		log.Print("baseline v2 reproduzida sem diferenças")
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(baselinePath), 0o755); err != nil {
		return fmt.Errorf("preparar pasta v2: %w", err)
	}
	if err := os.WriteFile(baselinePath, generated, 0o644); err != nil {
		return fmt.Errorf("escrever baseline v2: %w", err)
	}
	log.Printf("baseline v2 escrita em %s; revise o diff", baselinePath)
	return nil
}

func lookupColumns(table *schema.Table, names []string) ([]*schema.Column, error) {
	columns := make([]*schema.Column, 0, len(names))
	for _, name := range names {
		column, ok := table.Column(name)
		if !ok {
			return nil, fmt.Errorf("coluna %s.%s não encontrada", table.Name, name)
		}
		columns = append(columns, column)
	}
	return columns, nil
}

func quote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func quoteList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = quote(value)
	}
	return strings.Join(quoted, ", ")
}
