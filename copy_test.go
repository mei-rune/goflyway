package goflyway

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

func TestCopyMigrateTable_NormalCase(t *testing.T) {
	db, mock, _ := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	defer db.Close()

	tableNotExistErr := errors.New("pq: relation \"goose_versions\" does not exist")

	// EnsureDBVersion: 查询 goose 版本表（不存在，将创建）
	mock.ExpectQuery(`SELECT version_id, is_applied, description from goose_versions ORDER BY id DESC`).
		WillReturnError(tableNotExistErr)

	// createVersionTable: 事务 + CREATE TABLE + INSERT init version + COMMIT
	mock.ExpectBegin()
	mock.ExpectExec(`CREATE TABLE goose_versions (
					id serial NOT NULL,
					version_id bigint NOT NULL,
					is_applied boolean NOT NULL,
					description text,
					tstamp timestamp NULL default now(),
					PRIMARY KEY(id)
				);`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO goose_versions (version_id, is_applied, description) VALUES ($1, $2, $3);`).
		WithArgs(int64(0), true, "init version").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// getAllFlywayVersions: 查询 flyway 数据
	flywayRow := sqlmock.NewRows([]string{"version", "description", "installed_on"}).
		AddRow("1.2.030405", "Initial schema", time.Now())
	mock.ExpectQuery(`SELECT version, description, installed_on 
                          FROM flyway_schema 
                          ORDER BY installed_on ASC`).
		WillReturnRows(flywayRow)

	// InsertVersionSql: 插入 goose 版本记录 (postgres dialect, $1/$2/$3)
	mock.ExpectExec(`INSERT INTO goose_versions (version_id, is_applied, description) VALUES ($1, $2, $3);`).
		WithArgs(
			int64(20250102030405),
			true,
			"Initial schema",
		).WillReturnResult(sqlmock.NewResult(1, 1))

	err := CopyMigrateTable("postgres", db, "flyway_schema", "goose_versions", "2025")
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("未满足的数据库预期: %v", err)
	}
}

func TestInvalidTableNames(t *testing.T) {
	invalidTables := []string{"", "flyway!history", "goose;DROP TABLE users;"}
	for _, table := range invalidTables {
		t.Run(table, func(t *testing.T) {
			err := CopyMigrateTable("mysql", nil, table, "valid_table", "2025")
			if err == nil || !strings.Contains(err.Error(), "表名非法") {
				t.Errorf("未拒绝非法表名: %s", table)
			}
		})
	}
}

func TestEmptyFlywayTable(t *testing.T) {
	db, mock, _ := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))

	tableNotExistErr := errors.New("pq: relation \"goose_ver\" does not exist")

	// EnsureDBVersion: 查询 goose 版本表（不存在，将创建）
	mock.ExpectQuery(`SELECT version_id, is_applied, description from goose_ver ORDER BY id DESC`).
		WillReturnError(tableNotExistErr)

	// createVersionTable: 事务 + CREATE TABLE + INSERT init version + COMMIT
	mock.ExpectBegin()
	mock.ExpectExec(`CREATE TABLE goose_ver (
					id serial NOT NULL,
					version_id bigint NOT NULL,
					is_applied boolean NOT NULL,
					description text,
					tstamp timestamp NULL default now(),
					PRIMARY KEY(id)
				);`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO goose_ver (version_id, is_applied, description) VALUES ($1, $2, $3);`).
		WithArgs(int64(0), true, "init version").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// getAllFlywayVersions: 空结果集
	mock.ExpectQuery(`SELECT version, description, installed_on 
                          FROM flyway_history 
                          ORDER BY installed_on ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"version", "description", "installed_on"}))

	err := CopyMigrateTable("postgres", db, "flyway_history", "goose_ver", "2025")
	if err == nil || !strings.Contains(err.Error(), "无版本记录") {
		t.Error("未处理空表场景", err)
	}
}

func TestCrossDriver_DataConsistency_RealDrivers(t *testing.T) {
	pgDSN := "postgres://golang:123456@127.0.0.1:5432/golang?sslmode=disable"
	pgDB, err := sql.Open("postgres", pgDSN)
	if err != nil {
		t.Fatalf("PostgreSQL连接失败: %v", err)
	}
	defer pgDB.Close()

	_, err = pgDB.Exec(`CREATE TABLE IF NOT EXISTS flyway_schema
				(
				    installed_rank integer NOT NULL,
				    version character varying(50) ,
				    description character varying(200)  NOT NULL,
				    type character varying(20)  NOT NULL,
				    script character varying(1000)  NOT NULL,
				    checksum integer,
				    installed_by character varying(100)  NOT NULL,
				    installed_on timestamp without time zone NOT NULL DEFAULT now(),
				    execution_time integer NOT NULL,
				    success boolean NOT NULL,
				    CONSTRAINT schema_version_pk PRIMARY KEY (installed_rank)
				)
    `)
	if err != nil {
		t.Fatal("创建表失败:", err)
	}
	defer pgDB.Exec("DROP TABLE flyway_schema")
	defer pgDB.Exec("DROP TABLE goose_test_versions")

	now := time.Now().UTC()
	_, err = pgDB.Exec(`
        INSERT INTO flyway_schema (installed_rank, version, description, type, script, checksum, installed_by, installed_on, execution_time, success)
        VALUES 
            (1, '1.1', 'Initial migration', 'sql', 'V_1_1__Initial_migration.sql', 12, 'tpt', $1, 1, true),
            (2, '1.2', 'Add user table', 'sql', 'V_1_2__Add_user_table.sql', 12, 'tpt', $2, 1, true)
    `, now, now.Add(24*time.Hour))
	if err != nil {
		t.Fatal("写入数据失败:", err)
	}

	err = CopyMigrateTable("postgres", pgDB, "flyway_schema", "goose_test_versions", "2025")
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	flywayData := queryFlywayData(t, pgDB)
	gooseData := queryGooseData(t, pgDB)

	if len(flywayData) != len(gooseData) {
		t.Fatalf("记录数不一致: Flyway(%d) vs Goose(%d)", len(flywayData), len(gooseData))
	}

	for i := range flywayData {
		m := flywayData[i]
		p := gooseData[i]

		if m.VersionID != p.VersionID {
			t.Errorf("版本ID不一致: Flyway(%d) vs Goose(%d)", m.VersionID, p.VersionID)
		}
		if m.Description != p.Description {
			t.Errorf("描述不一致: Flyway(%s) vs Goose(%s)", m.Description, p.Description)
		}
		if !p.IsApplied {
			t.Errorf("状态字段错误: Goose is_applied=%v (预期=true)", p.IsApplied)
		}
	}
}

type FlywayRecord struct {
	VersionID   int64
	Description string
	InstalledOn time.Time
}

type GooseRecord struct {
	VersionID   int64
	Description string
	IsApplied   bool
	Tstamp      time.Time
}

func queryFlywayData(t *testing.T, db *sql.DB) []FlywayRecord {
	rows, err := db.Query(`
        SELECT 
            version, 
            description, 
            installed_on 
        FROM flyway_schema where installed_rank = 2
    `)
	if err != nil {
		t.Fatal("MySQL查询失败:", err)
	}
	defer rows.Close()

	var records []FlywayRecord
	for rows.Next() {
		var rawVersion string
		var r FlywayRecord
		if err := rows.Scan(&rawVersion, &r.Description, &r.InstalledOn); err != nil {
			t.Fatal("MySQL数据解析失败:", err)
		}
		timestampVersion, err := convertToGooseTimestamp(rawVersion, "2025")
		if err != nil {
			t.Fatal("版本转换失败:", err)
		}
		r.VersionID, err = strconv.ParseInt(timestampVersion, 10, 64)
		if err != nil {
			t.Fatal("版本转换失败:", err)
		}
		records = append(records, r)
	}
	return records
}

func queryGooseData(t *testing.T, db *sql.DB) []GooseRecord {
	rows, err := db.Query(`
        SELECT 
            version_id, 
            description, 
            is_applied, 
            tstamp 
        FROM goose_test_versions
    `)
	if err != nil {
		t.Fatal("PostgreSQL查询失败:", err)
	}
	defer rows.Close()

	var records []GooseRecord
	for rows.Next() {
		var r GooseRecord
		if err := rows.Scan(&r.VersionID, &r.Description, &r.IsApplied, &r.Tstamp); err != nil {
			t.Fatal("PostgreSQL数据解析失败:", err)
		}
		records = append(records, r)
	}
	return records
}
