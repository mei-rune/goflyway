package goflyway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/mei-rune/goose"
)

// CopyMigrateTable copies Flyway migration records into goose's version table.
func CopyMigrateTable(
	driver string,
	db *sql.DB,
	flywayTable string, // Flyway表名
	gooseTable string, // Goose表名
	baseYear string, // 年份
) error {
	// 1. 表名校验（防SQL注入）
	if err := goose.ValidateTableNames(flywayTable, gooseTable); err != nil {
		return fmt.Errorf("表名非法: %s", err)
	}

	// 2. 创建 goose Provider
	p, err := goose.NewProvider(&goose.DBConfig{
		DriverName: driver,
		TableName:  gooseTable,
	}, goose.WithConn(db))
	if err != nil {
		return fmt.Errorf("创建 goose provider 失败: %w", err)
	}

	// 3. 确保 goose 版本表存在
	if _, err := p.EnsureDBVersion(context.Background()); err != nil {
		return fmt.Errorf("ensure goose version table: %w", err)
	}

	// 4. 获取最新Flyway版本记录
	migrations, err := getAllFlywayVersions(db, flywayTable)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("Flyway表 %s 无版本记录", flywayTable)
		}
		return fmt.Errorf("读取Flyway版本失败: %s", err)
	}

	if len(migrations) == 0 {
		return fmt.Errorf("Flyway表 %s 无版本记录", flywayTable)
	}

	// 5. 逐条插入 goose 版本表
	for _, migration := range migrations {
		if migration.version == "" {
			return fmt.Errorf("Flyway表 %s 无版本记录", flywayTable)
		}

		timestampVersion, err := convertToGooseTimestamp(migration.version, baseYear)
		if err != nil {
			return fmt.Errorf("版本转换失败: %s", err)
		}
		versionID, err := strconv.ParseInt(timestampVersion, 10, 64)
		if err != nil {
			return fmt.Errorf("版本转换失败: %s", err)
		}

		if err := p.Dialect().InsertVersionSql(context.Background(), db, gooseTable, versionID, true, migration.desc); err != nil {
			return fmt.Errorf("插入版本 %d 失败: %w", versionID, err)
		}
	}

	return nil
}

type flywayMigrateResult struct {
	version     string
	desc        string
	installedOn time.Time
}

// getAllFlywayVersions reads all Flyway migration records in chronological order.
func getAllFlywayVersions(
	db *sql.DB,
	flywayTable string,
) ([]flywayMigrateResult, error) {
	query := fmt.Sprintf(`SELECT version, description, installed_on 
                          FROM %s 
                          ORDER BY installed_on ASC`, flywayTable)

	rows, err := db.Query(query)
	if err != nil {
		if goose.IsTableNotExists(err) {
			return nil, sql.ErrNoRows
		}
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

// IsTableAlreadyExists reports whether the given error indicates that
// a table already exists.
func IsTableAlreadyExists(err error) bool {
	return goose.IsTableAlreadyExists(err)
}
