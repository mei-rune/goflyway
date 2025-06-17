package goflyway

import (
	"io"
	"strings"
	"unicode"
)

var SqlHandleHooks []func(string) (string, error)

// ConvertFlywayToGoose 将 Flyway SQL 转换为 Goose SQL 格式
func ConvertFlywayToGoose(in io.Reader) (string, error) {
	// 分割 SQL 语句
	statements, err := Split(in)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	result.WriteString("-- +goose Up\n")

	for _, stmt := range statements {
		// 保留语句中的原始换行和缩进
		trimmedStmt := stmt

		for {
			idx := strings.IndexRune(trimmedStmt, '\n')
			if idx < 0 {
				break
			}
			line := trimmedStmt[:idx+1]
			if strings.TrimSpace(line) != "" {
				break
			}

			trimmedStmt = trimmedStmt[idx+1:]
			result.WriteString(line)
		}

		// 检查语句是否包含内部分号（除结尾分号外）
		hasInternalSemicolon := hasInternalSemicolon(trimmedStmt)

		for _, hook := range SqlHandleHooks {
			trimmedStmt, err = hook(trimmedStmt)
			if err != nil {
				return "", err
			}
		}

		////////////////////////////////////////////
		// 我自已有一部份老代码中有这个
		if strings.Contains(trimmedStmt, "-- +goose StatementBegin") {
			hasInternalSemicolon = false
		}
		////////////////////////////////////////////

		// 对于复杂语句，添加额外的 Goose 指令
		if hasInternalSemicolon {
			result.WriteString("\n-- +goose StatementBegin\n")
		}

		// 添加语句内容
		result.WriteString(trimmedStmt)

		// 对于复杂语句，添加结束指令
		if hasInternalSemicolon {
			result.WriteString("\n-- +goose StatementEnd")
		}

		result.WriteString("\n")
	}

	if len(statements) > 0 {
		// goose 要求 sql 必须有分号结束
		if !hasSemicolonAtEnt(statements[len(statements)-1]) {
			result.WriteString(";\n")
		}
	}

	result.WriteString("\n-- +goose Down")
	result.WriteString("\n-- Down migration is not supported in automatic conversion")
	result.WriteString("\n")
	return result.String(), nil
}

// hasInternalSemicolon 检查语句是否包含内部分号（非结尾分号）
func hasInternalSemicolon(stmt string) bool {
	// 去除尾部空白和分号
	trimmed := strings.TrimRightFunc(stmt, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ';'
	})

	// 检查剩余部分是否包含分号
	return strings.Contains(trimmed, ";")
}

func hasSemicolonAtEnt(stmt string) bool {
	// 去除尾部空白和分号
	trimmed := strings.TrimRightFunc(stmt, unicode.IsSpace)
	if trimmed == "" {
		return true
	}

	// 检查剩余部分是否包含分号
	return strings.HasSuffix(trimmed, ";")
}
