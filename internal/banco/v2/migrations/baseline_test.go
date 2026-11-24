package migrations_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const postgresImage = "postgres:18-alpine"

type model struct {
	Tables []modelTable `json:"tables"`
}

type modelTable struct {
	Name       string            `json:"name"`
	Columns    []modelColumn     `json:"columns"`
	Uniques    [][]string        `json:"uniques"`
	ForeignKey []modelForeignKey `json:"foreignKeys"`
}

type modelColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Primary  bool   `json:"primary"`
}

type modelForeignKey struct {
	Columns []string `json:"columns"`
	Table   string   `json:"table"`
	Target  []string `json:"target"`
}

func TestBaselineEmPostgreSQL18Vazio(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
	defer cancel()

	container, err := postgres.Run(ctx, postgresImage,
		postgres.WithDatabase("concierge_v2_test"),
		postgres.WithUsername("concierge_v2_test"),
		postgres.WithPassword("concierge_v2_test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("iniciar contêiner PostgreSQL 18 descartável: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Errorf("encerrar contêiner de teste próprio: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("obter conexão do contêiner de teste: %v", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("abrir banco do contêiner de teste: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("fechar banco de teste: %v", err)
		}
	})
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("conectar ao banco PostgreSQL 18 descartável: %v", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("."))
	if err != nil {
		t.Fatalf("abrir migrations Goose do diretório v2: %v", err)
	}
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("fechar provider Goose: %v", err)
		}
	})

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("aplicar baseline em banco novo: %v", err)
	}
	verifyBaseline(t, db)
	verifyConstraints(t, db)

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("reaplicar migrations sem mudanças: %v", err)
	}
	verifyBaseline(t, db)

	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("reverter baseline até a versão zero: %v", err)
	}
	assertNoBusinessTables(t, db)
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("reaplicar baseline depois de Down: %v", err)
	}
	verifyBaseline(t, db)
}

func verifyBaseline(t *testing.T, db *sql.DB) {
	t.Helper()
	m := loadModel(t)
	verifyTablesAndColumns(t, db, m)
	verifyPrimaryKeys(t, db, m)
	verifyUniqueIndexes(t, db, m)
	verifyForeignKeys(t, db, m)
}

func loadModel(t *testing.T) model {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("localizar arquivo de teste para abrir modelo.json")
	}
	path := filepath.Join(filepath.Dir(source), "../../../../docs/modelagem/concierge-v2/modelo.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler modelo de referência %s: %v", path, err)
	}
	var m model
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decodificar modelo de referência: %v", err)
	}
	return m
}

func verifyTablesAndColumns(t *testing.T, db *sql.DB, m model) {
	t.Helper()
	expectedTables := map[string]modelTable{}
	for _, table := range m.Tables {
		expectedTables[table.Name] = table
	}
	if len(expectedTables) != 24 {
		t.Fatalf("modelo de referência tem %d tabelas, esperava 24", len(expectedTables))
	}
	rows, err := db.Query(`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' AND table_name <> 'goose_db_version'`)
	if err != nil {
		t.Fatalf("listar tabelas public: %v", err)
	}
	actualTables := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			t.Fatalf("ler nome da tabela: %v", err)
		}
		actualTables[name] = true
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("fechar leitura de tabelas: %v", err)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("percorrer tabelas: %v", err)
	}
	if !reflect.DeepEqual(sortedKeys(actualTables), sortedModelTableNames(m)) {
		t.Fatalf("tabelas public divergentes; reais=%v modelo=%v", sortedKeys(actualTables), sortedModelTableNames(m))
	}

	actualColumns := map[string]modelColumn{}
	columnRows, err := db.Query(`
		SELECT c.table_name, c.column_name, pg_catalog.format_type(a.atttypid, a.atttypmod), NOT a.attnotnull
		FROM information_schema.columns c
		JOIN pg_catalog.pg_namespace n ON n.nspname = c.table_schema
		JOIN pg_catalog.pg_class r ON r.relnamespace = n.oid AND r.relname = c.table_name
		JOIN pg_catalog.pg_attribute a ON a.attrelid = r.oid AND a.attname = c.column_name AND NOT a.attisdropped
		WHERE c.table_schema = 'public' AND c.table_name <> 'goose_db_version'
	`)
	if err != nil {
		t.Fatalf("introspect colunas public: %v", err)
	}
	for columnRows.Next() {
		var table, name, typeName string
		var nullable bool
		if err := columnRows.Scan(&table, &name, &typeName, &nullable); err != nil {
			columnRows.Close()
			t.Fatalf("ler coluna: %v", err)
		}
		actualColumns[table+"."+name] = modelColumn{Name: name, Type: typeName, Nullable: nullable}
	}
	if err := columnRows.Close(); err != nil {
		t.Fatalf("fechar leitura de colunas: %v", err)
	}
	if err := columnRows.Err(); err != nil {
		t.Fatalf("percorrer colunas: %v", err)
	}

	expectedColumns := map[string]modelColumn{}
	for _, table := range m.Tables {
		for _, column := range table.Columns {
			expectedColumns[table.Name+"."+column.Name] = column
		}
	}
	if len(expectedColumns) != 197 {
		t.Fatalf("modelo de referência tem %d colunas, esperava 197", len(expectedColumns))
	}
	if !reflect.DeepEqual(sortedKeys(actualColumns), sortedKeys(expectedColumns)) {
		t.Fatalf("colunas divergentes; ausentes=%v extras=%v", setDifference(expectedColumns, actualColumns), setDifference(actualColumns, expectedColumns))
	}
	for key, expected := range expectedColumns {
		actual := actualColumns[key]
		if actual.Type != postgresType(expected.Type) || actual.Nullable != expected.Nullable {
			t.Errorf("%s: tipo/nullability reais %s nullable=%t, modelo %s nullable=%t", key, actual.Type, actual.Nullable, postgresType(expected.Type), expected.Nullable)
		}
	}
}

func verifyPrimaryKeys(t *testing.T, db *sql.DB, m model) {
	t.Helper()
	actual := map[string]string{}
	rows, err := db.Query(`
		SELECT r.relname, string_agg(a.attname, ',' ORDER BY k.ordinality)
		FROM pg_catalog.pg_constraint c
		JOIN pg_catalog.pg_class r ON r.oid = c.conrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = r.relnamespace
		CROSS JOIN LATERAL unnest(c.conkey) WITH ORDINALITY AS k(attnum, ordinality)
		JOIN pg_catalog.pg_attribute a ON a.attrelid = r.oid AND a.attnum = k.attnum
		WHERE n.nspname = 'public' AND c.contype = 'p' AND r.relname <> 'goose_db_version'
		GROUP BY r.relname
	`)
	if err != nil {
		t.Fatalf("introspect primary keys: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var table, columns string
		if err := rows.Scan(&table, &columns); err != nil {
			t.Fatalf("ler primary key: %v", err)
		}
		actual[table] = columns
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("percorrer primary keys: %v", err)
	}
	expected := map[string]string{}
	for _, table := range m.Tables {
		var columns []string
		for _, column := range table.Columns {
			if column.Primary {
				columns = append(columns, column.Name)
			}
		}
		if len(columns) > 0 {
			expected[table.Name] = strings.Join(columns, ",")
		}
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("primary keys divergentes; reais=%v modelo=%v", actual, expected)
	}
}

func verifyUniqueIndexes(t *testing.T, db *sql.DB, m model) {
	t.Helper()
	actual := map[string]bool{}
	nullsNotDistinct := map[string]bool{}
	rows, err := db.Query(`
		SELECT r.relname, string_agg(a.attname, ',' ORDER BY k.ordinality), i.indnullsnotdistinct
		FROM pg_catalog.pg_index i
		JOIN pg_catalog.pg_class r ON r.oid = i.indrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = r.relnamespace
		CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY AS k(attnum, ordinality)
		JOIN pg_catalog.pg_attribute a ON a.attrelid = r.oid AND a.attnum = k.attnum
		WHERE n.nspname = 'public' AND i.indisunique AND NOT i.indisprimary
		  AND i.indpred IS NULL AND i.indexprs IS NULL AND k.ordinality <= i.indnkeyatts
		GROUP BY r.relname, i.indexrelid, i.indnullsnotdistinct
	`)
	if err != nil {
		t.Fatalf("introspect unique indexes: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var table, columns string
		var nnd bool
		if err := rows.Scan(&table, &columns, &nnd); err != nil {
			t.Fatalf("ler unique index: %v", err)
		}
		key := table + "." + columns
		actual[key] = true
		nullsNotDistinct[key] = nnd
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("percorrer unique indexes: %v", err)
	}
	expected := map[string]bool{}
	nullable := map[string]map[string]bool{}
	for _, table := range m.Tables {
		columnNullable := map[string]bool{}
		for _, column := range table.Columns {
			columnNullable[column.Name] = column.Nullable
		}
		for _, columns := range table.Uniques {
			key := table.Name + "." + strings.Join(columns, ",")
			expected[key] = true
			nullable[key] = columnNullable
		}
	}
	if !reflect.DeepEqual(sortedKeys(actual), sortedKeys(expected)) {
		t.Fatalf("unique keys divergentes; ausentes=%v extras=%v", setDifference(expected, actual), setDifference(actual, expected))
	}
	for key := range expected {
		_, columnsText, _ := strings.Cut(key, ".")
		needsNND := false
		for _, column := range strings.Split(columnsText, ",") {
			if nullable[key][column] {
				needsNND = true
			}
		}
		if needsNND && !nullsNotDistinct[key] {
			t.Errorf("%s permite duplicar NULL; o desenho exige UNIQUE NULLS NOT DISTINCT", key)
		}
	}
}

func verifyForeignKeys(t *testing.T, db *sql.DB, m model) {
	t.Helper()
	type foreignKey struct {
		deleteType string
		updateType string
		setNull    string
	}
	actual := map[string]foreignKey{}
	rows, err := db.Query(`
		SELECT local.relname,
		       (SELECT string_agg(a.attname, ',' ORDER BY k.ordinality) FROM unnest(c.conkey) WITH ORDINALITY AS k(attnum, ordinality) JOIN pg_catalog.pg_attribute a ON a.attrelid = local.oid AND a.attnum = k.attnum),
		       target.relname,
		       (SELECT string_agg(a.attname, ',' ORDER BY k.ordinality) FROM unnest(c.confkey) WITH ORDINALITY AS k(attnum, ordinality) JOIN pg_catalog.pg_attribute a ON a.attrelid = target.oid AND a.attnum = k.attnum),
		       c.confdeltype::text, c.confupdtype::text,
		       COALESCE((SELECT string_agg(a.attname, ',' ORDER BY k.ordinality) FROM unnest(c.confdelsetcols) WITH ORDINALITY AS k(attnum, ordinality) JOIN pg_catalog.pg_attribute a ON a.attrelid = local.oid AND a.attnum = k.attnum), '')
		FROM pg_catalog.pg_constraint c
		JOIN pg_catalog.pg_class local ON local.oid = c.conrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = local.relnamespace
		JOIN pg_catalog.pg_class target ON target.oid = c.confrelid
		WHERE n.nspname = 'public' AND c.contype = 'f'
	`)
	if err != nil {
		t.Fatalf("introspect foreign keys: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var localTable, localColumns, targetTable, targetColumns, deleteType, updateType, setNull string
		if err := rows.Scan(&localTable, &localColumns, &targetTable, &targetColumns, &deleteType, &updateType, &setNull); err != nil {
			t.Fatalf("ler foreign key: %v", err)
		}
		key := fkKey(localTable, strings.Split(localColumns, ","), targetTable, strings.Split(targetColumns, ","))
		actual[key] = foreignKey{deleteType: deleteType, updateType: updateType, setNull: setNull}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("percorrer foreign keys: %v", err)
	}
	expected := map[string]bool{}
	for _, table := range m.Tables {
		for _, fk := range table.ForeignKey {
			expected[fkKey(table.Name, fk.Columns, fk.Table, fk.Target)] = true
		}
	}
	if len(expected) != 24 {
		t.Fatalf("modelo de referência tem %d FKs, esperava 24", len(expected))
	}
	if !reflect.DeepEqual(sortedKeys(actual), sortedKeys(expected)) {
		t.Fatalf("FKs divergentes; ausentes=%v extras=%v", setDifference(expected, actual), setDifference(actual, expected))
	}
	for key, fk := range actual {
		if strings.HasPrefix(key, "requests.company_id,source_session_id->sessions.") {
			if fk.deleteType != "n" || fk.setNull != "source_session_id" {
				t.Errorf("FK %s: ON DELETE = %q, colunas SET NULL = %q; esperado SET NULL apenas source_session_id", key, fk.deleteType, fk.setNull)
			}
			if fk.updateType != "a" {
				t.Errorf("FK %s: ON UPDATE = %q, esperado NO ACTION", key, fk.updateType)
			}
			continue
		}
		if fk.deleteType != "a" || fk.updateType != "a" || fk.setNull != "" {
			t.Errorf("FK %s tem ação inesperada: ON DELETE=%q ON UPDATE=%q SET NULL=%q; esperava NO ACTION", key, fk.deleteType, fk.updateType, fk.setNull)
		}
	}
}

func verifyConstraints(t *testing.T, db *sql.DB) {
	t.Helper()
	t.Run("catalogo_e_canais", func(t *testing.T) { verifyCatalogAndChannels(t, db) })
	t.Run("sessao_pedidos_e_memoria", func(t *testing.T) { verifySessionsRequestsAndMemory(t, db) })
	t.Run("escopos_e_nulls", func(t *testing.T) { verifyScopesAndNulls(t, db) })
	t.Run("camadas_e_acesso", func(t *testing.T) { verifyLayersAndAccess(t, db) })
}

func verifyCatalogAndChannels(t *testing.T, db *sql.DB) {
	t.Helper()
	seedCatalogAndChannels(t, db)
	if _, err := db.Exec(`INSERT INTO catalog_files (path, content, sha256, updated_at) VALUES ('agentes/teste/caso-id-duplicado.yaml', 'synthetic fixture', 'synthetic-hash', now())`); err != nil {
		t.Fatalf("inserir arquivo sintético para testar ID único: %v", err)
	}
	assertSQLState(t, db, "23505", `INSERT INTO use_cases (id, file_path, agent_path, name, summary, visibility, source_sha256, updated_at) VALUES ('case-other-id', 'agentes/teste/caso-a.yaml', 'agentes/teste', 'Outro', 'Resumo', 'PUBLIC', 'hash', now())`)
	assertSQLState(t, db, "23505", `INSERT INTO use_cases (id, file_path, agent_path, name, summary, visibility, source_sha256, updated_at) VALUES ('case-a', 'agentes/teste/caso-id-duplicado.yaml', 'agentes/teste', 'Outro', 'Resumo', 'PUBLIC', 'hash', now())`)
	assertSQLState(t, db, "23503", `INSERT INTO use_cases (id, file_path, agent_path, name, summary, visibility, source_sha256, updated_at) VALUES ('case-no-file', 'agentes/ausente/caso.yaml', 'agentes/ausente', 'Sem arquivo', 'Resumo', 'PUBLIC', 'hash', now())`)

	assertSQLState(t, db, "23505", `INSERT INTO channels (id, company_id, channel_type, provider, endpoint_identifier, external_channel_id, feature_mode, enabled, root_agent, updated_at) VALUES ('30000000-0000-0000-0000-000000000003', 'empresa-beta', 'WHATSAPP', 'ATENDE', '+155500000001', 'tenant-alpha', 'CUSTOM', true, 'agentes/concierge', now())`)
	assertSQLState(t, db, "23514", `INSERT INTO channels (id, company_id, channel_type, provider, endpoint_identifier, external_channel_id, feature_mode, enabled, root_agent, updated_at) VALUES ('30000000-0000-0000-0000-000000000004', '   ', 'WHATSAPP', 'ATENDE', '+155500000004', 'tenant-other', 'CUSTOM', true, 'agentes/concierge', now())`)
	assertSQLState(t, db, "23514", `INSERT INTO channels (id, company_id, channel_type, provider, endpoint_identifier, external_channel_id, feature_mode, enabled, root_agent, updated_at) VALUES ('30000000-0000-0000-0000-000000000005', 'empresa-alpha', 'WHATSAPP', 'ATENDE', '+155500000005', 'tenant-other', 'UNKNOWN', true, 'agentes/concierge', now())`)
	assertSQLState(t, db, "23514", `INSERT INTO channels (id, company_id, channel_type, provider, endpoint_identifier, external_channel_id, feature_mode, enabled, root_agent, updated_at) VALUES ('30000000-0000-0000-0000-000000000006', 'empresa-alpha', 'FAX', 'ATENDE', '+155500000006', 'tenant-other', 'CUSTOM', true, 'agentes/concierge', now())`)
	if _, err := db.Exec(`INSERT INTO channels (id, company_id, channel_type, provider, endpoint_identifier, external_channel_id, feature_mode, enabled, root_agent, updated_at) VALUES ('30000000-0000-0000-0000-000000000007', 'empresa-alpha', 'WHATSAPP', 'ATENDE', '+155500000011', NULL, 'CUSTOM', true, 'agentes/concierge', now())`); err != nil {
		t.Fatalf("inserir canal com tenant externo ausente: %v", err)
	}
	assertSQLState(t, db, "23505", `INSERT INTO channels (id, company_id, channel_type, provider, endpoint_identifier, external_channel_id, feature_mode, enabled, root_agent, updated_at) VALUES ('30000000-0000-0000-0000-000000000008', 'empresa-beta', 'WHATSAPP', 'ATENDE', '+155500000011', NULL, 'CUSTOM', true, 'agentes/concierge', now())`)
	assertSQLState(t, db, "23503", `INSERT INTO company_use_cases (id, company_id, use_case_id, enabled, default_available, updated_at) VALUES ('20000000-0000-0000-0000-000000000099', 'empresa-alpha', 'case-missing', true, true, now())`)
	assertSQLState(t, db, "23514", `INSERT INTO company_use_cases (id, company_id, use_case_id, enabled, default_available, updated_at) VALUES ('20000000-0000-0000-0000-000000000098', '  ', 'case-a', true, true, now())`)

	insertCompanyUseCase(t, db, "20000000-0000-0000-0000-000000000002", "empresa-alpha", "case-b", true, false)
	insertCompanyUseCase(t, db, "20000000-0000-0000-0000-000000000003", "empresa-beta", "case-a", true, true)
	insertChannelUseCase(t, db, "40000000-0000-0000-0000-000000000001", "empresa-alpha", "30000000-0000-0000-0000-000000000001", "20000000-0000-0000-0000-000000000001", true)
	insertChannelUseCase(t, db, "40000000-0000-0000-0000-000000000002", "empresa-alpha", "30000000-0000-0000-0000-000000000001", "20000000-0000-0000-0000-000000000002", false)
	insertChannelUseCase(t, db, "40000000-0000-0000-0000-000000000003", "empresa-beta", "30000000-0000-0000-0000-000000000002", "20000000-0000-0000-0000-000000000003", true)
	assertSQLState(t, db, "23503", `INSERT INTO channel_use_cases (id, company_id, channel_id, company_use_case_id, enabled, updated_at) VALUES ('40000000-0000-0000-0000-000000000004', 'empresa-alpha', '30000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000001', true, now())`)
	assertSQLState(t, db, "23503", `INSERT INTO channel_use_cases (id, company_id, channel_id, company_use_case_id, enabled, updated_at) VALUES ('40000000-0000-0000-0000-000000000005', 'empresa-beta', '30000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000001', true, now())`)
	assertSQLState(t, db, "23502", `INSERT INTO channel_use_cases (id, company_id, channel_id, company_use_case_id, enabled, updated_at) VALUES ('40000000-0000-0000-0000-000000000006', 'empresa-beta', '30000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000003', NULL, now())`)
	var trueOverrides, falseOverrides, absentOverrides int
	if err := db.QueryRow(`SELECT count(*) FILTER (WHERE enabled), count(*) FILTER (WHERE NOT enabled) FROM channel_use_cases WHERE channel_id = '30000000-0000-0000-0000-000000000001'`).Scan(&trueOverrides, &falseOverrides); err != nil {
		t.Fatalf("consultar overrides true/false: %v", err)
	}
	if trueOverrides != 1 || falseOverrides != 1 {
		t.Fatalf("true e false precisam persistir como linhas distintas: true=%d false=%d", trueOverrides, falseOverrides)
	}
	if err := db.QueryRow(`SELECT count(*) FROM channel_use_cases WHERE channel_id = '30000000-0000-0000-0000-000000000002' AND company_use_case_id = '20000000-0000-0000-0000-000000000001'`).Scan(&absentOverrides); err != nil {
		t.Fatalf("consultar ausência de override: %v", err)
	}
	if absentOverrides != 0 {
		t.Fatalf("ausência de override foi confundida com linha: %d", absentOverrides)
	}
}

func seedCatalogAndChannels(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, item := range []struct{ path, id string }{
		{"agentes/teste/caso-a.yaml", "case-a"},
		{"agentes/teste/caso-b.yaml", "case-b"},
	} {
		if _, err := db.Exec(`INSERT INTO catalog_files (path, content, sha256, updated_at) VALUES ($1, $2, $3, now())`, item.path, []byte("synthetic fixture"), "synthetic-hash"); err != nil {
			t.Fatalf("inserir catalog_files %s: %v", item.path, err)
		}
		if _, err := db.Exec(`INSERT INTO use_cases (id, file_path, agent_path, name, summary, visibility, source_sha256, updated_at) VALUES ($1, $2, 'agentes/teste', $3, 'Resumo sintético', 'PUBLIC', 'synthetic-hash', now())`, item.id, item.path, item.id); err != nil {
			t.Fatalf("inserir use_cases %s: %v", item.id, err)
		}
	}
	insertCompanyUseCase(t, db, "20000000-0000-0000-0000-000000000001", "empresa-alpha", "case-a", true, true)
	for _, channel := range []struct{ id, company, endpoint, tenant string }{
		{"30000000-0000-0000-0000-000000000001", "empresa-alpha", "+155500000001", "tenant-alpha"},
		{"30000000-0000-0000-0000-000000000002", "empresa-beta", "+155500000002", "tenant-beta"},
	} {
		_, err := db.Exec(`INSERT INTO channels (id, company_id, channel_type, provider, endpoint_identifier, external_channel_id, feature_mode, enabled, root_agent, updated_at) VALUES ($1, $2, 'WHATSAPP', 'ATENDE', $3, $4, 'CUSTOM', true, 'agentes/concierge', now())`, channel.id, channel.company, channel.endpoint, channel.tenant)
		if err != nil {
			t.Fatalf("inserir canal sintético %s: %v", channel.id, err)
		}
	}
}

func insertCompanyUseCase(t *testing.T, db *sql.DB, id, company, useCase string, enabled, available bool) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO company_use_cases (id, company_id, use_case_id, enabled, default_available, updated_at) VALUES ($1, $2, $3, $4, $5, now())`, id, company, useCase, enabled, available); err != nil {
		t.Fatalf("inserir company_use_cases %s: %v", id, err)
	}
}

func insertChannelUseCase(t *testing.T, db *sql.DB, id, company, channel, companyUseCase string, enabled bool) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO channel_use_cases (id, company_id, channel_id, company_use_case_id, enabled, updated_at) VALUES ($1, $2, $3, $4, $5, now())`, id, company, channel, companyUseCase, enabled); err != nil {
		t.Fatalf("inserir channel_use_cases %s: %v", id, err)
	}
}

func verifySessionsRequestsAndMemory(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO sessions (id, company_id, channel_id, connector, protocol, phone_number, condo_id, data, updated_at) VALUES ('session-alpha', 'empresa-alpha', '30000000-0000-0000-0000-000000000001', 'ATENDE', 'proto-alpha', '+155500000010', 101, '{}'::jsonb, now())`); err != nil {
		t.Fatalf("inserir sessão coerente: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (id, company_id, channel_id, connector, protocol, phone_number, condo_id, data, updated_at) VALUES ('session-beta', 'empresa-beta', '30000000-0000-0000-0000-000000000002', 'ATENDE', 'proto-beta', '+155500000020', NULL, '{}'::jsonb, now())`); err != nil {
		t.Fatalf("inserir sessão beta coerente com condo NULL: %v", err)
	}
	assertSQLState(t, db, "23503", `INSERT INTO sessions (id, company_id, channel_id, connector, protocol, phone_number, data, updated_at) VALUES ('session-cross-company', 'empresa-beta', '30000000-0000-0000-0000-000000000001', 'ATENDE', 'proto-cross', '+155500000030', '{}'::jsonb, now())`)

	if _, err := db.Exec(`INSERT INTO contact_memories (company_id, phone_number, name, updated_at) VALUES ('empresa-alpha', '+155500000010', 'Contato sintético', now()), ('empresa-beta', '+155500000010', 'Contato sintético', now())`); err != nil {
		t.Fatalf("inserir cabeçalhos de memória para duas empresas: %v", err)
	}
	assertSQLState(t, db, "23503", `INSERT INTO memory_facts (company_id, phone_number, condo_id, key, text, occurred_at) VALUES ('empresa-alpha', '+155500000099', 101, 'pet', 'cão', now())`)
	if _, err := db.Exec(`INSERT INTO memory_facts (company_id, phone_number, condo_id, key, text, occurred_at) VALUES ('empresa-alpha', '+155500000010', 101, 'pet', 'Alpha', now()), ('empresa-alpha', '+155500000010', 102, 'pet', 'Beta', now()), ('empresa-alpha', '+155500000010', NULL, 'geral', 'Geral', now())`); err != nil {
		t.Fatalf("inserir fatos Alpha/Beta e fato sem condo: %v", err)
	}
	assertSQLState(t, db, "23505", `INSERT INTO memory_facts (company_id, phone_number, condo_id, key, text, occurred_at) VALUES ('empresa-alpha', '+155500000010', 101, 'pet', 'Colisão Alpha', now())`)
	assertSQLState(t, db, "23505", `INSERT INTO memory_facts (company_id, phone_number, condo_id, key, text, occurred_at) VALUES ('empresa-alpha', '+155500000010', NULL, 'geral', 'Colisão sem condo', now())`)

	if _, err := db.Exec(`INSERT INTO requests (protocol, company_id, phone_number, condo_id, company_use_case_id, source_session_id, status, data, updated_at) VALUES ('pedido-alpha', 'empresa-alpha', '+155500000010', 101, '20000000-0000-0000-0000-000000000001', 'session-alpha', 'COLLECTING', '{}'::jsonb, now())`); err != nil {
		t.Fatalf("inserir pedido com empresa, caso e sessão coerentes: %v", err)
	}
	assertSQLState(t, db, "23503", `INSERT INTO requests (protocol, company_id, phone_number, company_use_case_id, status, data, updated_at) VALUES ('pedido-caso-cruzado', 'empresa-beta', '+155500000010', '20000000-0000-0000-0000-000000000001', 'COLLECTING', '{}'::jsonb, now())`)
	assertSQLState(t, db, "23503", `INSERT INTO requests (protocol, company_id, phone_number, company_use_case_id, source_session_id, status, data, updated_at) VALUES ('pedido-sessao-cruzada', 'empresa-alpha', '+155500000010', '20000000-0000-0000-0000-000000000001', 'session-beta', 'COLLECTING', '{}'::jsonb, now())`)
	if _, err := db.Exec(`INSERT INTO request_events (company_id, protocol, sequence, type, text, occurred_at) VALUES ('empresa-alpha', 'pedido-alpha', 1, 'CREATED', 'Criado', now())`); err != nil {
		t.Fatalf("inserir evento de pedido coerente: %v", err)
	}
	assertSQLState(t, db, "23503", `INSERT INTO request_events (company_id, protocol, sequence, type, text, occurred_at) VALUES ('empresa-beta', 'pedido-alpha', 2, 'UPDATED', 'Cruza empresa', now())`)

	assertSQLState(t, db, "23503", `DELETE FROM channels WHERE id = '30000000-0000-0000-0000-000000000001'`)
	assertSQLState(t, db, "23503", `DELETE FROM company_use_cases WHERE id = '20000000-0000-0000-0000-000000000001'`)
	if _, err := db.Exec(`DELETE FROM sessions WHERE id = 'session-alpha'`); err != nil {
		t.Fatalf("apagar sessão deve limpar só a origem opcional do pedido: %v", err)
	}
	var company, status string
	var sourceSession sql.NullString
	var data []byte
	if err := db.QueryRow(`SELECT company_id, status, source_session_id, data FROM requests WHERE protocol = 'pedido-alpha'`).Scan(&company, &status, &sourceSession, &data); err != nil {
		t.Fatalf("consultar pedido depois do reset da sessão: %v", err)
	}
	if company != "empresa-alpha" || status != "COLLECTING" || sourceSession.Valid || string(data) != "{}" {
		t.Fatalf("pedido após exclusão de sessão alterado indevidamente: company=%q status=%q source_session_id=%v data=%s", company, status, sourceSession, data)
	}
}

func verifyScopesAndNulls(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, fixture := range []struct {
		scope   string
		company any
		condo   any
		key     string
	}{
		{"GLOBAL", nil, nil, "resource"},
		{"COMPANY", "empresa-alpha", nil, "resource"},
		{"CONDO", "empresa-alpha", 101, "resource"},
	} {
		if _, err := db.Exec(`INSERT INTO payloads (scope_type, company_id, condo_id, resource_key, data) VALUES ($1, $2, $3, $4, '{}'::jsonb)`, fixture.scope, fixture.company, fixture.condo, fixture.key); err != nil {
			t.Fatalf("inserir payload %s: %v", fixture.scope, err)
		}
	}
	assertSQLState(t, db, "23514", `INSERT INTO payloads (scope_type, company_id, condo_id, resource_key, data) VALUES ('GLOBAL', 'empresa-alpha', NULL, 'invalid-global', '{}'::jsonb)`)
	assertSQLState(t, db, "23514", `INSERT INTO payloads (scope_type, company_id, condo_id, resource_key, data) VALUES ('COMPANY', 'empresa-alpha', 101, 'invalid-company', '{}'::jsonb)`)
	assertSQLState(t, db, "23514", `INSERT INTO payloads (scope_type, company_id, condo_id, resource_key, data) VALUES ('CONDO', 'empresa-alpha', NULL, 'invalid-condo', '{}'::jsonb)`)
	assertSQLState(t, db, "23514", `INSERT INTO payloads (scope_type, company_id, condo_id, resource_key, data) VALUES ('COMPANY', '   ', NULL, 'blank-company', '{}'::jsonb)`)
	assertSQLState(t, db, "23505", `INSERT INTO payloads (scope_type, company_id, condo_id, resource_key, data) VALUES ('GLOBAL', NULL, NULL, 'resource', '{}'::jsonb)`)

	for _, fixture := range []struct {
		scope   string
		company any
		condo   any
	}{
		{"GLOBAL", nil, nil},
		{"COMPANY", "empresa-alpha", nil},
		{"CONDO", "empresa-alpha", 101},
	} {
		if _, err := db.Exec(`INSERT INTO guidelines (scope_type, company_id, condo_id) VALUES ($1, $2, $3)`, fixture.scope, fixture.company, fixture.condo); err != nil {
			t.Fatalf("inserir guideline %s: %v", fixture.scope, err)
		}
	}
	assertSQLState(t, db, "23514", `INSERT INTO guidelines (scope_type, company_id, condo_id) VALUES ('GLOBAL', 'empresa-alpha', NULL)`)
	assertSQLState(t, db, "23514", `INSERT INTO guidelines (scope_type, company_id, condo_id) VALUES ('COMPANY', 'empresa-alpha', 101)`)
	assertSQLState(t, db, "23514", `INSERT INTO guidelines (scope_type, company_id, condo_id) VALUES ('CONDO', 'empresa-alpha', NULL)`)
	assertSQLState(t, db, "23514", `INSERT INTO guidelines (scope_type, company_id, condo_id) VALUES ('COMPANY', ' ', NULL)`)
	assertSQLState(t, db, "23505", `INSERT INTO guidelines (scope_type, company_id, condo_id) VALUES ('GLOBAL', NULL, NULL)`)

	if _, err := db.Exec(`INSERT INTO hook_rules (id, scope_type, company_id, condo_ids, version, name, event, enabled, definition, updated_at) VALUES ('50000000-0000-0000-0000-000000000001', 'GLOBAL', NULL, '{}', 1, 'Global', 'REQUEST', true, '{}'::jsonb, now()), ('50000000-0000-0000-0000-000000000002', 'COMPANY', 'empresa-alpha', '{101,102}', 1, 'Empresa', 'REQUEST', true, '{}'::jsonb, now())`); err != nil {
		t.Fatalf("inserir regras global e empresarial válidas: %v", err)
	}
	assertSQLState(t, db, "23514", `INSERT INTO hook_rules (id, scope_type, company_id, condo_ids, version, name, event, enabled, definition, updated_at) VALUES ('50000000-0000-0000-0000-000000000003', 'GLOBAL', 'empresa-alpha', '{}', 1, 'Global inválida', 'REQUEST', true, '{}'::jsonb, now())`)
	assertSQLState(t, db, "23514", `INSERT INTO hook_rules (id, scope_type, company_id, condo_ids, version, name, event, enabled, definition, updated_at) VALUES ('50000000-0000-0000-0000-000000000004', 'GLOBAL', NULL, '{101}', 1, 'Global inválida', 'REQUEST', true, '{}'::jsonb, now())`)
	assertSQLState(t, db, "23514", `INSERT INTO hook_rules (id, scope_type, company_id, condo_ids, version, name, event, enabled, definition, updated_at) VALUES ('50000000-0000-0000-0000-000000000005', 'COMPANY', '  ', '{}', 1, 'Empresa inválida', 'REQUEST', true, '{}'::jsonb, now())`)
	assertSQLState(t, db, "23514", `INSERT INTO hook_rules (id, scope_type, company_id, condo_ids, version, name, event, enabled, definition, updated_at) VALUES ('50000000-0000-0000-0000-000000000006', 'COMPANY', 'empresa-alpha', ARRAY[NULL]::integer[], 1, 'Empresa inválida', 'REQUEST', true, '{}'::jsonb, now())`)

	if _, err := db.Exec(`INSERT INTO hook_audit_logs (id, scope_type, origin, event, hook, decision, reason, created_at) VALUES ('60000000-0000-0000-0000-000000000001', 'GLOBAL', 'test', 'REQUEST', 'hook', 'ALLOW', 'synthetic', now())`); err != nil {
		t.Fatalf("inserir auditoria global sem escopo empresarial: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO hook_audit_logs (id, scope_type, company_id, condo_id, channel_id, origin, event, hook, decision, reason, created_at) VALUES ('60000000-0000-0000-0000-000000000002', 'COMPANY', 'empresa-alpha', 101, '30000000-0000-0000-0000-000000000001', 'test', 'REQUEST', 'hook', 'ALLOW', 'synthetic', now())`); err != nil {
		t.Fatalf("inserir auditoria empresarial ligada a canal da mesma empresa: %v", err)
	}
	assertSQLState(t, db, "23514", `INSERT INTO hook_audit_logs (id, scope_type, company_id, origin, event, hook, decision, reason, created_at) VALUES ('60000000-0000-0000-0000-000000000003', 'GLOBAL', 'empresa-alpha', 'test', 'REQUEST', 'hook', 'ALLOW', 'invalid', now())`)
	assertSQLState(t, db, "23514", `INSERT INTO hook_audit_logs (id, scope_type, condo_id, origin, event, hook, decision, reason, created_at) VALUES ('60000000-0000-0000-0000-000000000004', 'GLOBAL', 101, 'test', 'REQUEST', 'hook', 'ALLOW', 'invalid', now())`)
	assertSQLState(t, db, "23514", `INSERT INTO hook_audit_logs (id, scope_type, company_id, origin, event, hook, decision, reason, created_at) VALUES ('60000000-0000-0000-0000-000000000005', 'COMPANY', '   ', 'test', 'REQUEST', 'hook', 'ALLOW', 'invalid', now())`)

	var globalCount int
	if err := db.QueryRow(`SELECT count(*) FROM payloads WHERE scope_type = 'GLOBAL' AND company_id IS NULL AND condo_id IS NULL`).Scan(&globalCount); err != nil {
		t.Fatalf("consultar payload global com NULL: %v", err)
	}
	if globalCount != 1 {
		t.Fatalf("payload global NULL não persistiu: %d linhas", globalCount)
	}
}

func verifyLayersAndAccess(t *testing.T, db *sql.DB) {
	t.Helper()
	baseLayer := insertSkillLayer(t, db, "BASE", nil, nil)
	companyLayer := insertSkillLayer(t, db, "COMPANY", "empresa-alpha", nil)
	condoLayer := insertSkillLayer(t, db, "CONDO", "empresa-alpha", 101)
	if baseLayer == 0 || companyLayer == 0 || condoLayer == 0 {
		t.Fatal("IDs de camadas devem ser gerados")
	}
	assertSQLState(t, db, "23505", `INSERT INTO skill_layers (skill, type, company_id, condo_id, draft_version) VALUES ('skill-inherited', 'BASE', NULL, NULL, '')`)
	assertSQLState(t, db, "23514", `INSERT INTO skill_layers (skill, type, company_id, condo_id, draft_version) VALUES ('skill-invalid-base', 'BASE', 'empresa-alpha', NULL, '')`)
	assertSQLState(t, db, "23514", `INSERT INTO skill_layers (skill, type, company_id, condo_id, draft_version) VALUES ('skill-invalid-company', 'COMPANY', 'empresa-alpha', 101, '')`)

	baseVersion := insertSkillVersion(t, db, baseLayer, 1, nil)
	companyVersion := insertSkillVersion(t, db, companyLayer, 1, &baseVersion)
	companyVersionNext := insertSkillVersion(t, db, companyLayer, 2, &baseVersion)
	if _, err := db.Exec(`INSERT INTO skill_updates (layer_id, from_version_id, to_version_id, deadline_at, created_at) VALUES ($1, $2, $3, now(), now())`, companyLayer, companyVersion, companyVersionNext); err != nil {
		t.Fatalf("inserir atualização ligando revisões da camada: %v", err)
	}
	var updateID int64
	if err := db.QueryRow(`SELECT id FROM skill_updates WHERE layer_id = $1`, companyLayer).Scan(&updateID); err != nil {
		t.Fatalf("ler atualização: %v", err)
	}
	if _, err := db.Exec(`UPDATE skill_versions SET update_id = $1 WHERE id = $2`, updateID, companyVersionNext); err != nil {
		t.Fatalf("ligar revisão à atualização existente: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO skill_notices (layer_id, version_id, created_at) VALUES ($1, $2, now())`, companyLayer, companyVersionNext); err != nil {
		t.Fatalf("inserir aviso apontando para revisão existente: %v", err)
	}
	assertSQLState(t, db, "23503", `INSERT INTO skill_versions (layer_id, number, document, origin, author, created_at) VALUES ($1, 99, '{}'::jsonb, 'INITIAL', 'test', now())`, int64(999999))
	assertSQLState(t, db, "23503", `DELETE FROM skill_layers WHERE id = $1`, baseLayer)

	if _, err := db.Exec(`INSERT INTO users (id, name) VALUES ('operador-sintetico', 'Operador sintético')`); err != nil {
		t.Fatalf("inserir usuário operador sintético: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO roles (id) VALUES ('support'), ('reviewer')`); err != nil {
		t.Fatalf("inserir papéis sintéticos: %v", err)
	}
	if _, err := db.Exec(`UPDATE roles SET inherits_from = 'support' WHERE id = 'reviewer'`); err != nil {
		t.Fatalf("inserir herança de papel por FK: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO role_permissions (role_id, permission) VALUES ('support', 'studio.read')`); err != nil {
		t.Fatalf("inserir permissão base por FK: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO company_role_grants (company_id, role_id, permission) VALUES ('empresa-alpha', 'support', 'hooks.edit')`); err != nil {
		t.Fatalf("inserir concessão empresarial por FK: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO role_assignments (user_id, role_id, scope_type) VALUES ('operador-sintetico', 'support', 'PLATFORM')`); err != nil {
		t.Fatalf("inserir atribuição PLATFORM com NULLs válidos: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO role_assignments (user_id, role_id, scope_type, company_id) VALUES ('operador-sintetico', 'reviewer', 'COMPANY', 'empresa-alpha')`); err != nil {
		t.Fatalf("inserir atribuição COMPANY: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO role_assignments (user_id, role_id, scope_type, company_id, condo_id) VALUES ('operador-sintetico', 'support', 'CONDO', 'empresa-alpha', 101)`); err != nil {
		t.Fatalf("inserir atribuição CONDO: %v", err)
	}
	assertSQLState(t, db, "23514", `INSERT INTO role_assignments (user_id, role_id, scope_type, company_id) VALUES ('operador-sintetico', 'support', 'PLATFORM', 'empresa-alpha')`)
	assertSQLState(t, db, "23514", `INSERT INTO role_assignments (user_id, role_id, scope_type, company_id, condo_id) VALUES ('operador-sintetico', 'support', 'COMPANY', 'empresa-alpha', 101)`)
	assertSQLState(t, db, "23505", `INSERT INTO role_assignments (user_id, role_id, scope_type) VALUES ('operador-sintetico', 'support', 'PLATFORM')`)
	assertSQLState(t, db, "23503", `DELETE FROM roles WHERE id = 'support'`)
}

func insertSkillLayer(t *testing.T, db *sql.DB, kind string, company any, condo any) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO skill_layers (skill, type, company_id, condo_id, draft_version) VALUES ('skill-inherited', $1, $2, $3, '') RETURNING id`, kind, company, condo).Scan(&id); err != nil {
		t.Fatalf("inserir camada %s: %v", kind, err)
	}
	return id
}

func insertSkillVersion(t *testing.T, db *sql.DB, layer int64, number int, parent *int64) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO skill_versions (layer_id, number, document, parent_version_id, origin, author, created_at) VALUES ($1, $2, '{}'::jsonb, $3, 'INITIAL', 'test', now()) RETURNING id`, layer, number, parent).Scan(&id); err != nil {
		t.Fatalf("inserir revisão %d da camada %d: %v", number, layer, err)
	}
	return id
}

func assertSQLState(t *testing.T, db *sql.DB, state, query string, args ...any) {
	t.Helper()
	_, err := db.Exec(query, args...)
	if err == nil {
		t.Errorf("SQL deveria ser recusado com SQLSTATE %s, mas foi aceito: %s", state, query)
		return
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Errorf("SQL falhou fora do PostgreSQL; erro=%v query=%s", err, query)
		return
	}
	if pgErr.Code != state {
		t.Errorf("SQLSTATE real %s, esperado %s; mensagem=%s query=%s", pgErr.Code, state, pgErr.Message, query)
	}
}

func assertNoBusinessTables(t *testing.T, db *sql.DB) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' AND table_name <> 'goose_db_version'`).Scan(&count); err != nil {
		t.Fatalf("contar tabelas depois de Down: %v", err)
	}
	if count != 0 {
		t.Fatalf("Down deixou %d tabelas de negócio", count)
	}
}

func postgresType(modelType string) string {
	if modelType == "timestamptz" {
		return "timestamp with time zone"
	}
	return modelType
}

func fkKey(local string, localColumns []string, target string, targetColumns []string) string {
	return local + "." + strings.Join(localColumns, ",") + "->" + target + "." + strings.Join(targetColumns, ",")
}

func sortedModelTableNames(m model) []string {
	names := make([]string, 0, len(m.Tables))
	for _, table := range m.Tables {
		names = append(names, table.Name)
	}
	sort.Strings(names)
	return names
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func setDifference[L any, R any](left map[string]L, right map[string]R) []string {
	difference := make([]string, 0)
	for key := range left {
		if _, ok := right[key]; !ok {
			difference = append(difference, key)
		}
	}
	sort.Strings(difference)
	return difference
}
