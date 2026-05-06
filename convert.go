package goflyway

import (
	"io"
	"regexp"
	"strings"
)

var SqlHandleHooks []func(string) (string, error)

// 预编译的正则表达式，用于匹配 Goose StatementBegin/StatementEnd 指令（goose 部分可选）
var gooseStatementDirectiveRE = regexp.MustCompile(`(?i)--\s*\+(goose\s+)?Statement(Begin|End)`)

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
		// 使用正则表达式匹配，忽略大小写和空白字符变化
		if gooseStatementDirectiveRE.MatchString(trimmedStmt) {
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
		// 如果最后一个语句已经包含 Goose 指令，则不需要添加分号
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
	trimFunc := func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ';'
	}

	lines := strings.Split(stmt, "\n")
	offset := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "--") {
			break
		}
		offset++
	}

	// 去除尾部空白和分号
	trimmed := strings.TrimRightFunc(strings.Join(lines[offset:], "\n"), trimFunc)

	// 检查剩余部分是否包含分号
	return strings.Contains(trimmed, ";")
}

func hasSemicolonAtEnt(stmt string) bool {
	var lastNonCommentLine string
	for _, line := range strings.Split(stmt, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		lastNonCommentLine = trimmed
	}

	if lastNonCommentLine == "" {
		return true
	}

	return strings.HasSuffix(lastNonCommentLine, ";")
}
