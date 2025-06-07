package goflyway

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"unicode"
)

// TokenType 表示解析出的 token 类型
type TokenType int

const (
	TokenText TokenType = iota
	TokenSemicolon
	TokenBegin
	TokenEnd
	TokenDelimiterCommand
)

// Token 表示解析出的词法单元
type Token struct {
	Type          TokenType
	Value         string
	DelimiterWord string // 当前使用的分隔符
}

// Tokenizer 封装 SQL 解析器
type Tokenizer struct {
	reader *bufio.Reader
}

func NewTokenizer(in io.Reader) *Tokenizer {
	return &Tokenizer{reader: bufio.NewReader(in)}
}

func (t *Tokenizer) NextToken() (Token, error) {
	r, err := t.readRune()
	if err != nil {
		return Token{}, err // 返回错误（包括io.EOF）
	}

	switch {
	case r == '\'' || r == '"':
		return t.readQuotedString(r)

	case r == '-':
		if next, err := t.peekRune(); err != nil && err != io.EOF {
			return Token{Type: TokenText, Value: string(r)}, err
		} else if next == '-' {
			return t.readLineComment()
		}
		return Token{Type: TokenText, Value: string(r)}, nil

	case r == '/':
		if next, err := t.peekRune(); err != nil && err != io.EOF {
			return Token{Type: TokenText, Value: string(r)}, err
		} else if next == '*' {
			return t.readBlockComment()
		}
		return Token{Type: TokenText, Value: string(r)}, nil

	case r == ';':
		return Token{Type: TokenSemicolon, Value: ";"}, nil

	case unicode.IsLetter(r) || r == '_':
		return t.readWord(r)

	default:
		return Token{Type: TokenText, Value: string(r)}, nil
	}
}

func (t *Tokenizer) readQuotedString(quote rune) (Token, error) {
	var builder strings.Builder
	builder.WriteRune(quote)

	for {
		r, err := t.readRune()
		if err != nil {
			return Token{Type: TokenText, Value: builder.String()}, err
		}

		builder.WriteRune(r)

		if r == quote {
			if next, err := t.peekRune(); err != nil {
				if err != io.EOF {
					return Token{Type: TokenText, Value: builder.String()}, err
				}
				break
			} else if next == quote {
				if _, err := t.readRune(); err != nil {
					if err != io.EOF {
						return Token{Type: TokenText, Value: builder.String()}, err
					}
					builder.WriteRune(quote)
					break
				}
				builder.WriteRune(quote)
			} else {
				break
			}
		}
	}

	return Token{Type: TokenText, Value: builder.String()}, nil
}

func (t *Tokenizer) readLineComment() (Token, error) {
	var builder strings.Builder
	builder.WriteRune('-')

	second, err := t.readRune()
	if err != nil {
		if errors.Is(err, io.EOF) {
			err = nil
		}
		return Token{Type: TokenText, Value: builder.String()}, err
	}
	builder.WriteRune(second)

	for {
		r, err := t.readRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return Token{Type: TokenText, Value: builder.String()}, err
		}
		builder.WriteRune(r)
		if r == '\n' {
			break
		}
	}

	return Token{Type: TokenText, Value: builder.String()}, nil
}

func (t *Tokenizer) readBlockComment() (Token, error) {
	var builder strings.Builder
	builder.WriteRune('/')

	second, err := t.readRune()
	if err != nil {
		return Token{Type: TokenText, Value: builder.String()}, err
	}
	builder.WriteRune(second)

	for {
		r, err := t.readRune()
		if err != nil {
			return Token{Type: TokenText, Value: builder.String()}, err
		}

		builder.WriteRune(r)

		if r == '*' {
			if next, err := t.peekRune(); err != nil {
				if err != io.EOF {
					return Token{Type: TokenText, Value: builder.String()}, err
				}
				break
			} else if next == '/' {
				end, err := t.readRune()
				if err != nil {
					if err != io.EOF {
						return Token{Type: TokenText, Value: builder.String()}, err
					}
					break
				}
				builder.WriteRune(end)
				break
			}
		}
	}

	return Token{Type: TokenText, Value: builder.String()}, nil
}

// 读取块分隔符内的内容
func (t *Tokenizer) readUntilBlockDelimiter(tag string) (string, error) {
	endDelim := "$" + tag + "$"
	endRunes := []rune(endDelim)

	var builder strings.Builder
	window := make([]rune, len(endRunes))
	count := 0
	matched := false

	for {
		r, err := t.readRune()
		if err != nil {
			return builder.String(), err
		}

		builder.WriteRune(r)

		// 更新滑动窗口
		if count < len(endRunes) {
			window[count] = r
		} else {
			copy(window, window[1:])
			window[len(endRunes)-1] = r
		}
		count++

		// 检查是否匹配结束标记
		if count >= len(endRunes) {
			matched = true
			for i, rn := range window {
				if rn != endRunes[i] {
					matched = false
					break
				}
			}
			if matched {
				break
			}
		}
	}

	return builder.String(), nil
}

func (t *Tokenizer) readWord(first rune) (Token, error) {
	var builder strings.Builder
	builder.WriteRune(first)

	for {
		r, err := t.peekRune()
		if err != nil {
			if err != io.EOF {
				return Token{Type: TokenText, Value: builder.String()}, err
			}
			break
		}
		if !isWordRune(r) {
			break
		}
		if _, err := t.readRune(); err != nil {
			if err != io.EOF {
				return Token{Type: TokenText, Value: builder.String()}, err
			}
			builder.WriteRune(r)
			break
		}
		builder.WriteRune(r)
	}

	word := builder.String()
	upperWord := strings.ToUpper(word)

	switch upperWord {
	case "BEGIN":
		return Token{Type: TokenBegin, Value: word}, nil
	case "END":
		return Token{Type: TokenEnd, Value: word}, nil
	case "DELIMITER":
		return processDelimiterCommand(t.reader, word)
	case "AS":
		return t.processAsBlockStart(word)
	default:
		return Token{Type: TokenText, Value: word}, nil
	}
}

// 处理 AS 后的块分隔符开始
func (t *Tokenizer) processAsBlockStart(word string) (Token, error) {
	var result strings.Builder
	result.WriteString(word)

	// 跳过 AS 后面的空白字符
	for {
		peek, err := t.peekRune()
		if err != nil {
			if err != io.EOF {
				return Token{Type: TokenText, Value: result.String()}, err
			}
			break
		}
		if !unicode.IsSpace(peek) {
			break
		}
		r, err := t.readRune()
		if err != nil {
			if err != io.EOF {
				return Token{Type: TokenText, Value: result.String()}, err
			}
			result.WriteRune(r)
			break
		}
		result.WriteRune(r)
	}

	// 检查美元符号
	peek, err := t.peekRune()
	if err != nil {
		if err != io.EOF {
			return Token{Type: TokenText, Value: result.String()}, err
		}
		return Token{Type: TokenText, Value: result.String()}, nil
	}

	if peek == '$' {
		// 消耗美元符号
		dollar, err := t.readRune()
		if err != nil {
			if err != io.EOF {
				return Token{Type: TokenText, Value: result.String()}, err
			}
			result.WriteRune('$')
			return Token{Type: TokenText, Value: result.String()}, nil
		}
		result.WriteRune(dollar)

		// 读取标签
		tag, err := t.readUntilRune('$')
		if err != nil {
			if err != io.EOF {
				return Token{Type: TokenText, Value: result.String() + tag}, err
			}
			result.WriteString(tag)
			return Token{Type: TokenText, Value: result.String()}, nil
		}

		// 添加标签到结果
		result.WriteString(tag)

		// 消耗结束美元符号
		endDollar, err := t.readRune()
		if err != nil {
			if err != io.EOF {
				return Token{Type: TokenText, Value: result.String()}, err
			}
			return Token{Type: TokenText, Value: result.String()}, nil
		}
		result.WriteRune(endDollar)

		// 读取整个块内容
		blockContent, err := t.readUntilBlockDelimiter(tag)
		if err != nil {
			if err != io.EOF {
				return Token{Type: TokenText, Value: result.String()}, err
			}
			result.WriteString(blockContent)
			return Token{Type: TokenText, Value: result.String()}, nil
		}

		// 添加块内容到结果
		result.WriteString(blockContent)

		return Token{
			Type:  TokenText,
			Value: result.String(),
		}, nil
	}

	// 没有找到块分隔符，返回普通文本
	return Token{Type: TokenText, Value: result.String()}, nil
}

// 读取直到指定字符（包含在值中），但将分隔符放回输入流
func (t *Tokenizer) readUntilRune(delim rune) (string, error) {
	var builder strings.Builder
	for {
		r, err := t.readRune()
		if err != nil {
			return builder.String(), err
		}
		if r == delim {
			// 将分隔符放回输入流
			if err := t.reader.UnreadRune(); err != nil {
				return builder.String(), err
			}
			return builder.String(), nil
		}
		builder.WriteRune(r)
	}
}

func processDelimiterCommand(in *bufio.Reader, commandStart string) (Token, error) {
	var builder strings.Builder
	builder.WriteString(commandStart)

	for {
		r, _, err := in.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return Token{Type: TokenDelimiterCommand, Value: builder.String()}, err
		}
		builder.WriteRune(r)
		if r == '\n' || r == ';' {
			break
		}
	}

	fullCommand := builder.String()
	delimPart := strings.TrimSpace(strings.TrimPrefix(fullCommand, "DELIMITER"))
	return Token{
		Type:          TokenDelimiterCommand,
		Value:         fullCommand,
		DelimiterWord: strings.TrimSpace(delimPart),
	}, nil
}

func (t *Tokenizer) readRune() (rune, error) {
	r, _, err := t.reader.ReadRune()
	return r, err
}

func (t *Tokenizer) peekRune() (rune, error) {
	r, _, err := t.reader.ReadRune()
	if err != nil {
		return 0, err
	}
	if err := t.reader.UnreadRune(); err != nil {
		return 0, err
	}
	return r, nil
}

// readUntilDelimiter 读取直到遇到自定义分隔符
func readUntilDelimiter(reader *bufio.Reader, delim string) (string, string, error) {
	var builder strings.Builder

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			return builder.String(), delim, err
		}
		builder.WriteRune(r)

		s := builder.String()
		if strings.HasSuffix(s, delim) {
			s := strings.TrimSuffix(s, delim)
			return s, delim, nil
		}

		if strings.HasSuffix(strings.ToUpper(s), "DELIMITER") {
			tmp := s[:len(s) - len("DELIMITER")]
			if tmp != "" {
				last := tmp[len(tmp)-1]
				if last != ';' && !unicode.IsSpace(rune(last)) {
					continue
				}
			}

			r, _, err := reader.ReadRune()
			if err != nil {
				return tmp, ";", err
			}
			if r == ';' {
				return tmp, ";", nil
			}
			if !unicode.IsSpace(r) {
				reader.UnreadRune()
				continue
			}

			token, err := processDelimiterCommand(reader, "DELIMITER")
			if err != nil {
				return tmp, delim, err
			}

			return tmp, token.DelimiterWord, nil
		}
	}
}

// Split 分割 SQL 语句
func Split(in io.Reader) ([]string, error) {
	tokenizer := NewTokenizer(in)
	var statements []string
	var stmtBuilder strings.Builder
	beginDepth := 0
	currentDelim := ";"

	for {
		// 自定义分隔符模式优先
		if currentDelim != ";" {
			content, nextDelim, err := readUntilDelimiter(tokenizer.reader, currentDelim)
			if err != nil {
				if err == io.EOF {
					if content != "" {
						stmtBuilder.WriteString(content)
					}
					if stmtBuilder.Len() > 0 {
						statements = append(statements, stmtBuilder.String())
						stmtBuilder.Reset()
					}
					break
				}
				return nil, err
			}

			stmtBuilder.WriteString(content)

			if s := stmtBuilder.String(); strings.TrimSpace(s) != "" {
				statements = append(statements, stmtBuilder.String())
			}
			stmtBuilder.Reset()

			currentDelim = nextDelim
			continue
		}

		// 标准模式
		token, err := tokenizer.NextToken()
		if err != nil {
			if err == io.EOF {
				break // 正常结束
			}
			return nil, err
		}

		switch token.Type {
		case TokenSemicolon:
			if beginDepth == 0 {
				// 关键修复：将分号添加到当前语句
				stmtBuilder.WriteString(token.Value)

				// 添加完整的语句
				if stmtBuilder.Len() > 0 {
					statements = append(statements, stmtBuilder.String())
					stmtBuilder.Reset()
				}
			} else {
				// BEGIN/END 块内的分号只是语句的一部分
				stmtBuilder.WriteString(token.Value)
			}

		case TokenBegin:
			beginDepth++
			stmtBuilder.WriteString(token.Value)

		case TokenEnd:
			if beginDepth > 0 {
				beginDepth--
			}
			stmtBuilder.WriteString(token.Value)

		case TokenDelimiterCommand:
			currentDelim = token.DelimiterWord
			stmtBuilder.Reset()
			// stmtBuilder.WriteString(token.Value)

		case TokenText:
			stmtBuilder.WriteString(token.Value)
		}
	}

	// 添加最后一条语句（如果存在）
	if stmtBuilder.Len() > 0 {
		statements = append(statements, stmtBuilder.String())
	}

	return statements, nil
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
