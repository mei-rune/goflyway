package goflyway

import (
	"database/sql"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

func TestCopyMigrateTable_NormalCase(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	mock.ExpectExec(`CREATE TABLE goose_versions ( id BIGINT AUTO_INCREMENT PRIMARY KEY, version_id BIGINT NOT NULL, is_applied TINYINT DEFAULT 1 NOT NULL, -- 默认标记为已应用 tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP, description VARCHAR(255) )`).WillReturnResult(sqlmock.NewResult(1, 1))

	// 模拟 Flyway 表数据（单条记录）
	flywayRow := sqlmock.NewRows([]string{"version", "description", "installed_on"}).
		AddRow("2025.01.02.030405", "Initial schema", time.Now())
	mock.ExpectQuery(`SELECT version, description, installed_on FROM flyway_schema`).
		WillReturnRows(flywayRow)

	// 预期 Goose 表操作
	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS goose_versions`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT 1 FROM goose_versions WHERE version_id = ?`).
		WithArgs(int64(20250102030405)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"})) // 无冲突
	mock.ExpectExec(`INSERT INTO goose_versions .+ VALUES .+`).
		WithArgs(
			int64(20250102030405), // 版本号
			1,                     // is_applied=1
			sqlmock.AnyArg(),      // tstamp (动态时间)
			"Initial schema",      // 描述
		).WillReturnResult(sqlmock.NewResult(1, 1))

	// 执行函数
	err := CopyMigrateTable("mysql", db, "flyway_schema", "goose_versions", "2025")
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	// 验证所有数据库操作被执行
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

func TestVersionConflict(t *testing.T) {
	db, mock, _ := sqlmock.New()
	mock.ExpectExec(`CREATE TABLE goose_versions ( id BIGINT AUTO_INCREMENT PRIMARY KEY, version_id BIGINT NOT NULL, is_applied TINYINT DEFAULT 1 NOT NULL, -- 默认标记为已应用 tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP, description VARCHAR(255) )`).WillReturnResult(sqlmock.NewResult(1, 1))

	// 模拟 Flyway 返回有效版本
	mock.ExpectQuery(`SELECT version, description, installed_on`).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("2025.01.01.000000"))
	// 模拟 Goose 表已存在该版本
	mock.ExpectQuery(`SELECT 1 FROM goose_versions`).
		WithArgs(int64(20250101000000)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))

	err := CopyMigrateTable("postgres", db, "flyway_tbl", "goose_tbl", "2025")
	if err == nil || !strings.Contains(err.Error(), "版本已存在") {
		t.Error("未检测到版本冲突")
	}
}

func TestEmptyFlywayTable(t *testing.T) {
	db, mock, _ := sqlmock.New()
	mock.ExpectExec(`CREATE TABLE goose_versions ( id BIGINT AUTO_INCREMENT PRIMARY KEY, version_id BIGINT NOT NULL, is_applied TINYINT DEFAULT 1 NOT NULL, -- 默认标记为已应用 tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP, description VARCHAR(255) )`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT version, description, installed_on`).
		WillReturnRows(sqlmock.NewRows([]string{"version"})) // 空结果集

	err := CopyMigrateTable("mysql", db, "flyway_history", "goose_ver", "2025")
	if err == nil || !strings.Contains(err.Error(), "无版本记录") {
		t.Error("未处理空表场景")
	}
}

func TestCrossDriver_DataConsistency_RealDrivers(t *testing.T) {
	// 1. 初始化真实数据库连接
	mysqlDSN := "user:password@tcp(192.168.1.102:3306)/flyway_db?parseTime=true"
	mysqlDB, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		t.Fatalf("MySQL连接失败: %v", err)
	}
	defer mysqlDB.Close()

	pgDSN := "postgres://golang:123456@127.0.0.1:5432/golang?sslmode=disable"
	pgDB, err := sql.Open("postgres", pgDSN)
	if err != nil {
		t.Fatalf("PostgreSQL连接失败: %v", err)
	}
	defer pgDB.Close()

	// 2. 准备测试数据（写入MySQL源表）
	_, err = mysqlDB.Exec(`
        CREATE TABLE IF NOT EXISTS flyway_schema (
            version VARCHAR(20) PRIMARY KEY,
            description TEXT,
            installed_on DATETIME
        )
    `)
	if err != nil {
		t.Fatal("创建MySQL表失败:", err)
	}
	defer mysqlDB.Exec("DROP TABLE flyway_schema")

	now := time.Now().UTC()
	_, err = mysqlDB.Exec(`
        INSERT INTO flyway_schema (version, description, installed_on)
        VALUES 
            ('2025.01.01', 'Initial migration', ?),
            ('2025.01.02', 'Add user table', ?)
    `, now, now.Add(24*time.Hour))
	if err != nil {
		t.Fatal("写入MySQL数据失败:", err)
	}

	// 3. 执行迁移（MySQL → PostgreSQL）
	err = CopyMigrateTable("mysql", mysqlDB, "flyway_schema", "goose_versions", "2025")
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	// 4. 查询并比对数据
	mysqlData := queryMySQLData(t, mysqlDB)
	pgData := queryPGData(t, pgDB)

	if len(mysqlData) != len(pgData) {
		t.Fatalf("记录数不一致: MySQL(%d) vs PostgreSQL(%d)", len(mysqlData), len(pgData))
	}

	for i := range mysqlData {
		m := mysqlData[i]
		p := pgData[i]

		// 核心字段比对
		if m.VersionID != p.VersionID {
			t.Errorf("版本ID不一致: MySQL(%d) vs PostgreSQL(%d)", m.VersionID, p.VersionID)
		}
		if m.Description != p.Description {
			t.Errorf("描述不一致: MySQL(%s) vs PostgreSQL(%s)", m.Description, p.Description)
		}
		if p.IsApplied != 1 {
			t.Errorf("状态字段错误: PostgreSQL is_applied=%d (预期=1)", p.IsApplied)
		}
		if m.InstalledOn.UTC() != p.Tstamp.UTC() {
			t.Errorf("时间戳不一致: MySQL(%v) vs PostgreSQL(%v)", m.InstalledOn, p.Tstamp)
		}
	}
}

// 数据模型
type MySQLRecord struct {
	VersionID   int64 // 转换后的版本号（如20250101）
	Description string
	InstalledOn time.Time
}

type PGRecord struct {
	VersionID   int64
	Description string
	IsApplied   int
	Tstamp      time.Time
}

// 从MySQL查询原始数据
func queryMySQLData(t *testing.T, db *sql.DB) []MySQLRecord {
	rows, err := db.Query(`
        SELECT 
            version, 
            description, 
            installed_on 
        FROM flyway_schema
    `)
	if err != nil {
		t.Fatal("MySQL查询失败:", err)
	}
	defer rows.Close()

	var records []MySQLRecord
	for rows.Next() {
		var rawVersion string
		var r MySQLRecord
		if err := rows.Scan(&rawVersion, &r.Description, &r.InstalledOn); err != nil {
			t.Fatal("MySQL数据解析失败:", err)
		}
		// 转换版本格式（"2025.01.01" → 20250101）
		// 4. 语义化版本 → 时间戳版本号
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

// 从PostgreSQL查询迁移后数据
func queryPGData(t *testing.T, db *sql.DB) []PGRecord {
	rows, err := db.Query(`
        SELECT 
            version_id, 
            description, 
            is_applied, 
            tstamp 
        FROM goose_versions
    `)
	if err != nil {
		t.Fatal("PostgreSQL查询失败:", err)
	}
	defer rows.Close()

	var records []PGRecord
	for rows.Next() {
		var r PGRecord
		if err := rows.Scan(&r.VersionID, &r.Description, &r.IsApplied, &r.Tstamp); err != nil {
			t.Fatal("PostgreSQL数据解析失败:", err)
		}
		records = append(records, r)
	}
	return records
}
