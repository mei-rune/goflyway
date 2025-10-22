package goflyway

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func IsTableAlreadyExists(err error) bool {
	return strings.Contains(err.Error(), "already exists") ||
	strings.Contains(err.Error(), "已存在")  ||
	strings.Contains(err.Error(), "已经存在") 
}

// 重命名函数：CopyMigrateTable
func CopyMigrateTable(
	driver string,
	db *sql.DB,
	flywayTable string, // Flyway表名
	gooseTable string, // Goose表名
	baseYear string, // 年份
) error {
	// 1. 表名校验（防SQL注入）
	if err := validateTableNames(flywayTable, gooseTable); err != nil {
		return fmt.Errorf("表名非法: %s", err)
	}

	// 2. 创建Goose版本表（若不存在）
	if err := createGooseTable(db, driver, gooseTable); err != nil {
		return fmt.Errorf("创建Goose表失败: %s", err)
	}

	// 3. 获取最新Flyway版本记录
	migrations, err := getAllFlywayVersions(db, driver, flywayTable)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("Flyway表 %s 无版本记录", flywayTable)
		}
		return fmt.Errorf("读取Flyway版本失败: %w", err)
	}
	for _, migration := range migrations {
		if migration.version == "" {
			return fmt.Errorf("Flyway表 %s 无版本记录", flywayTable)
		}

		// 4. 语义化版本 → 时间戳版本号
		timestampVersion, err := convertToGooseTimestamp(migration.version, baseYear)
		if err != nil {
			return fmt.Errorf("版本转换失败: %w", err)
		}
		versionID, err := strconv.ParseInt(timestampVersion, 10, 64)
		if err != nil {
			return fmt.Errorf("版本转换失败: %w", err)
		}

		// 5. 插入Goose版本表
		err = insertGooseVersion(db, driver, gooseTable, versionID, migration.installedOn, migration.desc)
		if err != nil {
			return err
		}
	}

	return nil
}

// 表名校验（正则验证）
func validateTableNames(tables ...string) error {
	validPattern := regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`) // 小写字母+下划线
	for _, tbl := range tables {
		if !validPattern.MatchString(tbl) {
			return fmt.Errorf("表名 %q 不符合命名规范", tbl)
		}
	}
	return nil
}

// 动态创建Goose表
func createGooseTable(db *sql.DB, driver, gooseTable string) error {
	var createSQL string
	switch driver {
	case "mysql":
		createSQL = fmt.Sprintf(`CREATE TABLE %s (
      id BIGINT AUTO_INCREMENT PRIMARY KEY,
      version_id BIGINT NOT NULL,
      is_applied TINYINT DEFAULT 1 NOT NULL, -- 默认标记为已应用
      tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      description VARCHAR(255)
    )`, gooseTable)
	case "postgres":
		createSQL = fmt.Sprintf(`CREATE TABLE %s (
      id BIGSERIAL PRIMARY KEY,
      version_id BIGINT NOT NULL,
      is_applied BOOLEAN DEFAULT TRUE NOT NULL,
      tstamp TIMESTAMPTZ DEFAULT NOW(),
      description TEXT
    )`, gooseTable)
	default:
		return fmt.Errorf("不支持的数据库类型: %s", driver)
	}
	_, err := db.Exec(createSQL)
	return err
}

type flywayMigrateResult struct {
	version string
	desc string
	installedOn time.Time
}

// 获取最新Flyway版本（安全查询）
func getAllFlywayVersions(
	db *sql.DB,
	driver string,
	flywayTable string,
) ([]flywayMigrateResult, error) {
	// 使用参数化避免SQL注入（表名已校验）
	query := fmt.Sprintf(`SELECT version, description, installed_on 
                          FROM %s 
                          ORDER BY installed_on ASC`, flywayTable)

  rows, err := db.Query(query)
  if err != nil {
  	return nil, err
  }
  defer rows.Close()

  var results []flywayMigrateResult
  for rows.Next() {
  	var result flywayMigrateResult
		err := rows.Scan(&result.version, &result.desc, &result.installedOn)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// 插入Goose版本记录
func insertGooseVersion(
	db *sql.DB,
	driver string,
	gooseTable string,
	version int64,
	t time.Time,
	desc string,
) error {
	// 动态生成插入语句
	var insertSQL string
	var args []interface{}
	switch driver {
	case "mysql":
		insertSQL = fmt.Sprintf(`INSERT INTO %s 
      (version_id, is_applied, tstamp, description) 
      VALUES (?, ?, ?, ?)`, gooseTable)
		args = []interface{}{version, 1, t.UTC(), desc}
	case "postgres":
		insertSQL = fmt.Sprintf(`INSERT INTO %s 
      (version_id, is_applied, tstamp, description) 
      VALUES ($1, $2, $3, $4)`, gooseTable)
		args = []interface{}{version, true, t, desc}
	}

	_, err := db.Exec(insertSQL, args...)
	if err != nil {
		return fmt.Errorf("插入失败: %w", err)
	}
	return nil
}
