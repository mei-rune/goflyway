package goflyway

import (
	"bufio"
	"io"
	"log"
	"strings"
)

// Split the given sql script into individual statements.
//
// The base case is to simply split on semicolons, as these
// naturally terminate a statement.
//
// However, more complex cases like pl/pgsql can have semicolons
// within a statement. For these cases, we provide the explicit annotations
// 'StatementBegin' and 'StatementEnd' to allow the script to
// tell us to ignore semicolons.
func splitByDelimiter(r io.Reader) (stmts []string, tokens []bool) {
	return SplitByDelimiter(r, "goose")
}

func SplitByDelimiter(r io.Reader, prefix string) ([]string, []bool) {
	var buf strings.Builder
	scanner := bufio.NewScanner(r)

	isFirst := true
	inStatementBlock := false
	var stmts []string
	var blocks []bool

	for scanner.Scan() {
		text := scanner.Text()

		if line := strings.TrimSpace(text); strings.HasPrefix(line, "--") {
			ss := strings.Fields(line)
			var cmd string
			if prefix == "" {
				if len(ss) == 2 {
					// -- +StatementBegin
					cmd = strings.TrimPrefix(ss[1], "+")
				}
			} else {
				if len(ss) == 3 && (ss[1] == prefix || ss[1] == "+"+prefix) {
					// -- +goose StatementBegin
					cmd = ss[2]
				} else if len(ss) == 2 {
					// -- +StatementBegin
					cmd = strings.TrimPrefix(ss[1], "+")
				}
			}

			// handle any goose-specific commands
			if cmd == "StatementBegin" || cmd == "statementBegin" {
				s := strings.TrimSpace(buf.String())
				if s != "" {
					stmts = append(stmts, buf.String())
					blocks = append(blocks, false)
				}

				buf.Reset()
				buf.WriteString(text)

				isFirst = false
				inStatementBlock = true
				continue
			}
			if cmd == "StatementEnd" || cmd == "statementEnd" {
				if !isFirst {
					buf.WriteString("\n")
				}

				buf.WriteString(text)

				stmts = append(stmts, buf.String())
				blocks = append(blocks, true)
				buf.Reset()

				isFirst = true
				inStatementBlock = false
				continue
			}
		}

		if isFirst {
			isFirst = false
		} else {
			buf.WriteString("\n")
		}

		if _, err := buf.WriteString(text); err != nil {
			log.Fatalf("io err: %v", err)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("scanning migration: %v", err)
	}

	if buf.Len() > 0 {
		stmt := strings.TrimSpace(buf.String())
		if stmt != "" {
			stmts = append(stmts, buf.String())
			blocks = append(blocks, false)
		}
	}

	// diagnose likely migration script errors
	if inStatementBlock {
		log.Println("WARNING: saw '-- +gobatis StatementBegin' with no matching '-- +gobatis StatementEnd'")
	}

	return stmts, blocks
}

func isEmptyOrComments(block string) bool {
	block = strings.TrimSpace(block)
	if block == "" {
		return true
	}

	lines := strings.Split(block, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "--") {
			return false
		}
	}
	return true
}
