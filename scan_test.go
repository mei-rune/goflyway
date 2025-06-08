package goflyway

import (
	"strings"
	"testing"
)

func TestConvertFlywayToGoose(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// 基础测试用例
		{
			name: "simple function",
			input: `CREATE FUNCTION test() RETURNS void AS $$
BEGIN
END;
$$ LANGUAGE plpgsql;`,
			expected: `-- +goose Up

-- +goose StatementBegin
CREATE FUNCTION test() RETURNS void AS $$
BEGIN
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- Down migration is not supported in automatic conversion
`,
		},
		// 复杂函数定义测试
		{
			name: "complex function with OR REPLACE",
			input: `CREATE OR REPLACE FUNCTION public.test_func(param1 int, param2 text)
RETURNS TABLE (id int, name text) AS $BODY$
DECLARE
    var1 text := 'test';
BEGIN
    -- 注释
    RETURN QUERY SELECT id, name FROM users WHERE active = true;
END;
$BODY$ LANGUAGE plpgsql;`,
			expected: `-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION public.test_func(param1 int, param2 text)
RETURNS TABLE (id int, name text) AS $BODY$
DECLARE
    var1 text := 'test';
BEGIN
    -- 注释
    RETURN QUERY SELECT id, name FROM users WHERE active = true;
END;
$BODY$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- Down migration is not supported in automatic conversion
`,
		},
		// 空格和格式变化测试
		{
			name: "whitespace variations",
			input: `CREATE  OR   REPLACE   PROCEDURE  test()
AS  $$  
BEGIN
    PERFORM something();  
END;  
$$  ;`,
			expected: `-- +goose Up

-- +goose StatementBegin
CREATE  OR   REPLACE   PROCEDURE  test()
AS  $$  
BEGIN
    PERFORM something();  
END;  
$$  ;
-- +goose StatementEnd

-- +goose Down
-- Down migration is not supported in automatic conversion
`,
		},
		// 大小写混合测试
		{
			name: "mixed case keywords",
			input: `CrEaTe Or RePlAcE FuNcTiOn test() 
ReTuRnS void AS $$
BeGiN
    -- code
EnD;
$$ lAnGuAgE plpgsql;`,
			expected: `-- +goose Up

-- +goose StatementBegin
CrEaTe Or RePlAcE FuNcTiOn test() 
ReTuRnS void AS $$
BeGiN
    -- code
EnD;
$$ lAnGuAgE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- Down migration is not supported in automatic conversion
`,
		},
		// 注释处理测试
		{
			name: "comments handling",
			input: `-- 头部注释
CREATE /* 块注释 */ FUNCTION test() -- 行注释
RETURNS void AS $BODY$
/* 多行
   注释 */
BEGIN
    -- 行内注释
    PERFORM something();
END;
$BODY$ LANGUAGE plpgsql;`,
			expected: `-- +goose Up

-- +goose StatementBegin
-- 头部注释
CREATE /* 块注释 */ FUNCTION test() -- 行注释
RETURNS void AS $BODY$
/* 多行
   注释 */
BEGIN
    -- 行内注释
    PERFORM something();
END;
$BODY$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- Down migration is not supported in automatic conversion
`,
		},
		// 字符串字面量测试
		{
			name: "string literals",
			input: `CREATE FUNCTION test() RETURNS void AS $$
BEGIN
    RAISE NOTICE 'This is a ''string'' with quotes';
    IF 'condition' THEN
        PERFORM something();
    END IF;
END;
$$ LANGUAGE plpgsql;`,
			expected: `-- +goose Up

-- +goose StatementBegin
CREATE FUNCTION test() RETURNS void AS $$
BEGIN
    RAISE NOTICE 'This is a ''string'' with quotes';
    IF 'condition' THEN
        PERFORM something();
    END IF;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- Down migration is not supported in automatic conversion
`,
		},
		// 多个函数测试
		{
			name: "multiple functions",
			input: `CREATE FUNCTION func1() RETURNS void AS $$
BEGIN
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION func2() RETURNS int AS $$
BEGIN
    RETURN 1;
END;
$$ LANGUAGE plpgsql;`,
			expected: `-- +goose Up

-- +goose StatementBegin
CREATE FUNCTION func1() RETURNS void AS $$
BEGIN
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd



-- +goose StatementBegin
CREATE OR REPLACE FUNCTION func2() RETURNS int AS $$
BEGIN
    RETURN 1;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- Down migration is not supported in automatic conversion
`,
		},
		// 非函数SQL测试
		{
			name: "non-function SQL",
			input: `CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);

INSERT INTO users (name) VALUES ('test');`,
			expected: `-- +goose Up
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);


INSERT INTO users (name) VALUES ('test');

-- +goose Down
-- Down migration is not supported in automatic conversion
`,
		},
		// 混合函数和非函数SQL测试
		{
			name: "mixed function and non-function SQL",
			input: `CREATE TABLE users (
    id SERIAL PRIMARY KEY
);

CREATE FUNCTION update_users() RETURNS void AS $$
BEGIN
    UPDATE users SET updated_at = NOW();
END;
$$ LANGUAGE plpgsql;

INSERT INTO users DEFAULT VALUES;`,
			expected: `-- +goose Up
CREATE TABLE users (
    id SERIAL PRIMARY KEY
);



-- +goose StatementBegin
CREATE FUNCTION update_users() RETURNS void AS $$
BEGIN
    UPDATE users SET updated_at = NOW();
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd


INSERT INTO users DEFAULT VALUES;

-- +goose Down
-- Down migration is not supported in automatic conversion
`,
		},
		// BEGIN/END嵌套测试
		{
			name: "nested BEGIN/END blocks",
			input: `CREATE FUNCTION test() RETURNS void AS $$
BEGIN
    BEGIN
        BEGIN
            PERFORM something();
        EXCEPTION WHEN OTHERS THEN
            RAISE NOTICE 'Error';
        END;
    END;
END;
$$ LANGUAGE plpgsql;`,
			expected: `-- +goose Up

-- +goose StatementBegin
CREATE FUNCTION test() RETURNS void AS $$
BEGIN
    BEGIN
        BEGIN
            PERFORM something();
        EXCEPTION WHEN OTHERS THEN
            RAISE NOTICE 'Error';
        END;
    END;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- Down migration is not supported in automatic conversion
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result, err := ConvertFlywayToGoose(reader)
			if err != nil {
				t.Fatalf("ConvertFlywayToGoose() error = %v", err)
			}

			if result != tt.expected {
				t.Errorf("ConvertFlywayToGoose() mismatch:\nExpected:\n%s\n\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestConvertFlywayToGoose_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// 空输入测试
		{
			name:     "empty input",
			input:    "",
			expected: "-- +goose Up\n\n-- +goose Down\n-- Down migration is not supported in automatic conversion\n",
		},
		// 只有注释的输入
		{
			name: "comments only",
			input: `-- 只有注释
/* 块注释 */`,
			expected: "-- +goose Up\n-- 只有注释\n/* 块注释 */\n;\n\n-- +goose Down\n-- Down migration is not supported in automatic conversion\n",
		},
		// 函数定义中没有分号
		{
			name: "function without semicolon",
			input: `CREATE FUNCTION test() RETURNS void AS $$
BEGIN
END
$$ LANGUAGE plpgsql`,
			expected: "-- +goose Up\nCREATE FUNCTION test() RETURNS void AS $$\nBEGIN\nEND\n$$ LANGUAGE plpgsql\n;\n\n-- +goose Down\n-- Down migration is not supported in automatic conversion\n",
		},
		// 函数定义中有多个分号
		{
			name: "multiple semicolons in function",
			input: `CREATE FUNCTION test() RETURNS void AS $$
BEGIN
    PERFORM func1();
    PERFORM func2();
END;
$$ LANGUAGE plpgsql;`,
			expected: "-- +goose Up\n\n-- +goose StatementBegin\nCREATE FUNCTION test() RETURNS void AS $$\nBEGIN\n    PERFORM func1();\n    PERFORM func2();\nEND;\n$$ LANGUAGE plpgsql;\n-- +goose StatementEnd\n\n-- +goose Down\n-- Down migration is not supported in automatic conversion\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result, err := ConvertFlywayToGoose(reader)
			if err != nil {
				t.Fatalf("ConvertFlywayToGoose() error = %v", err)
			}

			if result != tt.expected {
				t.Errorf("ConvertFlywayToGoose() mismatch:\nExpected:\n%s\n\nGot:\n%s", tt.expected, result)
			}
		})
	}
}
