package goflyway

import (
	"reflect"
	"strings"
	"testing"
	"unicode"
)

func TestMultipleStatements(t *testing.T) {
	input := "SELECT 1; SELECT 2;"
	expected := []string{"SELECT 1;", " SELECT 2;"}
	result, _ := Split(strings.NewReader(input))
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected: %v, Got: %v", expected, result)
	}

	commentLine := "-- abc;\n"
	input = "SELECT 1; SELECT 2;"
	result, _ = Split(strings.NewReader(commentLine + commentLine + input))

	expected = []string{commentLine + commentLine + "SELECT 1;", " SELECT 2;"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected: %v, Got: %v", expected, result)
	}

	input1 := "SELECT 1;"
	input2 := "SELECT 2;"

	result, _ = Split(strings.NewReader(commentLine + commentLine + input1 + "\n" + commentLine + input2 + commentLine))
	expected = []string{commentLine + commentLine + input1, "\n" + commentLine + input2, strings.TrimSpace(commentLine)}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected: %v, Got: %v", expected, result)
		for _, s := range result {
			t.Error(s)
		}
	}
}

func TestFunctionWithSemicolon(t *testing.T) {
	input := `CREATE FUNCTION update() RETURNS void AS $$
              BEGIN
                UPDATE users SET updated_at = NOW();
              END;
              $$ LANGUAGE plpgsql;`
	expected := []string{strings.TrimSpace(input)}
	result, _ := Split(strings.NewReader(input))
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected: %v, Got: %v", expected, result)
	}
}

func TestBeginEndBlock(t *testing.T) {
	input := `BEGIN; 
              INSERT INTO logs VALUES (1); 
              COMMIT; 
              END;`
	expected := []string{strings.TrimSpace(input)}
	result, _ := Split(strings.NewReader(input))
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected: %v, Got: %v", expected, result)
	}
}

func TestEmptyInput(t *testing.T) {
	input := ""
	result, err := Split(strings.NewReader(input))
	if err != nil || len(result) != 0 {
		t.Errorf("Expected empty slice, Got: %v", result)
	}
}

func TestNoSemicolon(t *testing.T) {
	input := "SELECT * FROM users"
	expected := []string{"SELECT * FROM users"}
	result, _ := Split(strings.NewReader(input))
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected: %v, Got: %v", expected, result)
	}
}

func TestTrailingChars(t *testing.T) {
	input := "SELECT 1; -- Comment"
	expected := []string{"SELECT 1;", " -- Comment"}
	result, _ := Split(strings.NewReader(input))
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected: %v, Got: %v", expected, result)
	}
}

func TestCustomDelimiter(t *testing.T) {
	input := "DELIMITER //\nCALL proc()//\nDELIMITER ;"
	expected := []string{"CALL proc()"}
	result, _ := Split(strings.NewReader(input))
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected: %v, Got: %v", expected, result)
		t.Error(len(result))
	}
}

func TestCustomDelimiter2(t *testing.T) {
	input := "DELIMITER //\nCALLDELIMITER proc()//\nDELIMITER ;"
	expected := []string{"CALLDELIMITER proc()"}
	result, _ := Split(strings.NewReader(input))
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected: %v, Got: %v", expected, result)
		t.Error(len(result))
	}
}

func TestCustomDelimiter3(t *testing.T) {
	input := "DELIMITER //\nDELIMITER33 proc()//\nDELIMITER ;"
	expected := []string{"DELIMITER33 proc()"}
	result, _ := Split(strings.NewReader(input))
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected: %v, Got: %v", expected, result)
		t.Error(len(result))
	}
}

func TestQuotedSemicolon(t *testing.T) {
	input := `INSERT INTO table VALUES ('text;here');`
	expected := []string{input}
	result, _ := Split(strings.NewReader(input))
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected: %v, Got: %v", expected, result)
	}
}

func TestSplitWithStatementSplit(t *testing.T) {
	input := `DROP FUNCTION IF EXISTS XXXX1;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION XXXX(TEXT)
RETURNS VOID AS $function$
BEGIN
	XXXX
END;
$function$ language plpgsql;
-- +goose StatementEnd

DROP FUNCTION IF EXISTS XXXX1;
`
	expected := []string{
		`DROP FUNCTION IF EXISTS XXXX1;`,
		`-- +goose StatementBegin
CREATE OR REPLACE FUNCTION XXXX(TEXT)
RETURNS VOID AS $function$
BEGIN
	XXXX
END;
$function$ language plpgsql;
-- +goose StatementEnd`,
		`DROP FUNCTION IF EXISTS XXXX1;`,
	}
	result, _ := Split(strings.NewReader(input))

	if len(result) != len(expected) {
		t.Errorf("Expected: %v, Got: %v", len(expected), len(result))
		for _, s := range result {
			t.Error(s)
		}
		return
	}
	for idx := range result {
		if strings.TrimSpace(result[idx]) != strings.TrimSpace(expected[idx]) {
			t.Errorf("Expected: %v, Got: %v", strings.TrimSpace(expected[idx]), strings.TrimSpace(result[idx]))
		}
	}
}

func normalizeWhitespace(s string) string {
	var result strings.Builder
	inSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !inSpace {
				result.WriteRune(' ')
				inSpace = true
			}
		} else {
			result.WriteRune(r)
			inSpace = false
		}
	}
	return strings.TrimSpace(result.String())
}

func TestSplit1(t *testing.T) {
	result, _ := Split(strings.NewReader(test_text1))

	expected := []string{
		`-- DROP TABLE if exists tpt_migrate_logs;

create table if not exists tpt_migrate_logs (
  id                 INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  name               varchar(250)  NOT NULL,
  logtext            TEXT, 
  executed           boolean DEFAULT '0',
  created_at         timestamp
);`,
		`DROP PROCEDURE IF EXISTS tpt_delete_mo_trigger;`,
		`-- +goose StatementBegin
-- 需要注意的是当前 mysql 不支持在 PREPARE 语句中执行 CREATE TRIGGER 和 DROP TRIGGER
-- 请见 https://dev.mysql.com/worklog/task/?id=2871
-- 所以下列 tpt_delete_mo_trigger 不可被调用，调用会返回 SQL 错误 [1295] [HY000]: This command is not supported in the prepared statement protocol yet
-- 为兼容性和可读性，我在程序中找到它，替换它
CREATE PROCEDURE tpt_delete_mo_trigger(tablename text) -- $1 is tablename
BEGIN
    SET @SQLSTR = CONCAT("DROP TRIGGER  IF EXISTS ", tablename, "_trigger_on_updated");
    set @LOGNAME = CONCAT('del ', tablename, '_trigger_on_updated');
    insert into tpt_migrate_logs (name, logtext, created_at) values(@LOGNAME, @SQLSTR, NOW());
    -- PREPARE u_trstmt FROM @SQLSTR;
    -- EXECUTE u_trstmt;

    SET @SQLSTR = CONCAT('DROP TRIGGER  IF EXISTS ', tablename, '_trigger_on_deleted;');
    set @LOGNAME = CONCAT('del ', tablename, '_trigger_on_deleted');
    insert into tpt_migrate_logs (name, logtext, created_at) values(@LOGNAME, @SQLSTR, NOW());
    -- PREPARE d_trstmt FROM @SQLSTR;
    -- EXECUTE d_trstmt;

    SET @SQLSTR = CONCAT('DROP TRIGGER  IF EXISTS ', tablename, '_trigger_on_creating;');
    set @LOGNAME = CONCAT('del ', tablename, '_trigger_on_creating');
    insert into tpt_migrate_logs (name, logtext, created_at) values(@LOGNAME, @SQLSTR, NOW());
    -- PREPARE c_trstmt FROM @SQLSTR;
    -- EXECUTE c_trstmt;
END;
-- +goose StatementEnd`,
		`DROP PROCEDURE IF EXISTS tpt_create_mo_trigger;`,
		`-- +goose StatementBegin
-- 需要注意的是当前 mysql 不支持在 PREPARE 语句中执行 CREATE TRIGGER 和 DROP TRIGGER
-- 请见 https://dev.mysql.com/worklog/task/?id=2871
-- 所以下列 tpt_delete_mo_trigger 不可被调用，调用会返回 SQL 错误 [1295] [HY000]: This command is not supported in the prepared statement protocol yet
-- 为兼容性和可读性，我在程序中找到它，替换它
CREATE PROCEDURE tpt_create_mo_trigger(tablename TEXT, objectType TEXT, fieldName TEXT) -- $1 is tablename, $2 is type
BEGIN
  SET @SQLSTR = CONCAT('CREATE TRIGGER ', tablename, '_trigger_on_creating
      BEFORE INSERT ON ', tablename, '
      FOR EACH ROW
      BEGIN
       IF NEW.created_at IS NULL THEN
           SET NEW.created_at = NOW();
        END IF;
        IF NEW.updated_at IS NULL THEN
           SET NEW.updated_at = NOW();
        END IF;
        
        INSERT INTO tpt_objects(name, type, table_name, created_at, updated_at)
          VALUES (NEW.display_name, ', objectType, ', ''', tablename, ''', NEW.created_at, NEW.updated_at);

        SET NEW.id =  LAST_INSERT_ID();
      END;');
  set @LOGNAME = CONCAT('create ', tablename, '_trigger_on_creating');
  insert into tpt_migrate_logs (name, logtext, created_at) values(@LOGNAME, @SQLSTR, NOW());
  -- PREPARE c_trstmt FROM @SQLSTR;
  -- EXECUTE c_trstmt;

  SET @SQLSTR = CONCAT('CREATE TRIGGER ', tablename, '_trigger_on_updated
      BEFORE UPDATE ON ', tablename, '
      FOR EACH ROW
      BEGIN
        IF NEW.updated_at IS NULL THEN
           NEW.updated_at = NOW();
        END IF;
       
        UPDATE  tpt_objects set name=NEW.', fieldName, ', updated_at=NEW.updated_at WHERE id = NEW.id;
      END;');
  set @LOGNAME = CONCAT('create ', tablename, '_trigger_on_updated');
  insert into tpt_migrate_logs (name, logtext, created_at) values(@LOGNAME, @SQLSTR, NOW());
  -- PREPARE u_trstmt FROM @SQLSTR;
  -- EXECUTE u_trstmt;

  SET @SQLSTR = CONCAT('CREATE TRIGGER ', tablename, '_trigger_on_deleted
      AFTER DELETE ON ', tablename, '
      FOR EACH ROW
      BEGIN
        DELETE FROM tpt_objects cascade WHERE id = OLD.id;
      END;');
  set @LOGNAME = CONCAT('del ', tablename, '_trigger_on_deleted');
    insert into tpt_migrate_logs (name, logtext, created_at) values(@LOGNAME, @SQLSTR, NOW());
  -- PREPARE d_trstmt FROM @SQLSTR;
  -- EXECUTE d_trstmt;
END;
-- +goose StatementEnd`,
		`DROP TABLE IF EXISTS tpt_domains CASCADE;`,
		`CREATE TABLE tpt_domains (
  id                 INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  name               varchar(50) NOT NULL,
  description        varchar(500),
  uuid               varchar(50) NOT NULL,
  created_at         datetime,
  updated_at         timestamp,

  unique(uuid)
);`,
		`DROP TABLE IF EXISTS tpt_engine_nodes  CASCADE;`,
		`CREATE TABLE tpt_engine_nodes (
  id                     integer NOT NULL AUTO_INCREMENT PRIMARY KEY,

  name                   varchar(250) NOT NULL,
  description            varchar(2000),
  fields                 json,

  created_at    datetime,
  updated_at    timestamp,

  UNIQUE(name)
);`,
		`INSERT INTO tpt_engine_nodes(name, description, created_at, updated_at) VALUES('default', '默认的采集引擎', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);`,
		`DROP TABLE IF EXISTS tpt_objects  CASCADE;`,
		`CREATE TABLE tpt_objects (
  id           integer NOT NULL AUTO_INCREMENT PRIMARY KEY,
  name         varchar(1000),
  type         varchar(200) NOT NULL,
  table_name   varchar(200)  NOT NULL,

  created_at    datetime,
  updated_at    timestamp
);
`,
		`
-- CREATE TABLE tpt_managed_objects (
--   id           integer PRIMARY KEY,
--   display_name varchar(200),
--   description  varchar(2000),
--   created_at    datetime,
--   updated_at    timestamp,

--   FOREIGN KEY(id) REFERENCES tpt_objects(id)  on delete cascade
-- );`,
	}

	if len(result) != len(expected) {
		t.Errorf("Expected %d statements, got %d", len(expected), len(result))
		for i, s := range result {
			t.Errorf("%d: %q", i, s)
		}
		return
	}

	for idx := range result {
		normalizedResult := normalizeWhitespace(result[idx])
		normalizedExpected := normalizeWhitespace(expected[idx])
		if normalizedResult != normalizedExpected {
			t.Errorf("Statement %d mismatch", idx)
			t.Errorf("Expected (normalized):\n%s", normalizedExpected)
			t.Errorf("Got (normalized):\n%s", normalizedResult)
		}
	}
}

func TestSplit2(t *testing.T) {
	result, _ := Split(strings.NewReader(test_text2))

	expected := []string{
		`DROP TABLE IF EXISTS tpt_domains CASCADE;`,
		`CREATE TABLE tpt_domains ( 
   id                 INT NOT NULL AUTO_INCREMENT PRIMARY KEY, 
   name               varchar(50) NOT NULL, 
   description        varchar(500), 
   uuid               varchar(50) NOT NULL, 
   created_at         datetime, 
   updated_at         timestamp, 

   unique(uuid) 
 );`,
		`DROP TABLE IF EXISTS tpt_engine_nodes  CASCADE;`,
		`CREATE TABLE tpt_engine_nodes ( 
   id                     integer NOT NULL AUTO_INCREMENT PRIMARY KEY, 

   name                   varchar(250) NOT NULL, 
   description            varchar(2000), 
   fields                 json, 

   created_at    datetime, 
   updated_at    timestamp, 

   UNIQUE(name) 
 );`,
		`INSERT INTO tpt_engine_nodes(name, description, created_at, updated_at) VALUES('default', '默认的采集引擎', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);`,
		`DROP TABLE IF EXISTS tpt_objects  CASCADE;`,
		`CREATE TABLE tpt_objects ( 
   id           integer NOT NULL AUTO_INCREMENT PRIMARY KEY, 
   name         varchar(1000), 
   type         varchar(200) NOT NULL, 
   table_name   varchar(200)  NOT NULL, 

   created_at    datetime, 
   updated_at    timestamp 
 );`,
		`
-- CREATE TABLE tpt_managed_objects ( 
--   id           integer PRIMARY KEY, 
--   display_name varchar(200), 
--   description  varchar(2000), 
--   created_at    datetime, 
--   updated_at    timestamp, 

--   FOREIGN KEY(id) REFERENCES tpt_objects(id)  on delete cascade 
-- );`,
	}

	if len(result) != len(expected) {
		t.Errorf("Expected %d statements, got %d", len(expected), len(result))
		for i, s := range result {
			t.Errorf("%d: %q", i, s)
		}
		return
	}

	for idx := range result {
		normalizedResult := normalizeWhitespace(result[idx])
		normalizedExpected := normalizeWhitespace(expected[idx])
		if normalizedResult != normalizedExpected {
			t.Errorf("Statement %d mismatch", idx)
			t.Errorf("Expected (normalized):\n%s", normalizedExpected)
			t.Errorf("Got (normalized):\n%s", normalizedResult)
		}
	}
}

const test_text2 = `delimiter ; 

DROP TABLE IF EXISTS tpt_domains CASCADE; 

CREATE TABLE tpt_domains ( 
   id                 INT NOT NULL AUTO_INCREMENT PRIMARY KEY, 
   name               varchar(50) NOT NULL, 
   description        varchar(500), 
   uuid               varchar(50) NOT NULL, 
   created_at         datetime, 
   updated_at         timestamp, 

   unique(uuid) 
 ); 




DROP TABLE IF EXISTS tpt_engine_nodes  CASCADE; 

CREATE TABLE tpt_engine_nodes ( 
   id                     integer NOT NULL AUTO_INCREMENT PRIMARY KEY, 

   name                   varchar(250) NOT NULL, 
   description            varchar(2000), 
   fields                 json, 

   created_at    datetime, 
   updated_at    timestamp, 

   UNIQUE(name) 
 ); 

INSERT INTO tpt_engine_nodes(name, description, created_at, updated_at) VALUES('default', '默认的采集引擎', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP); 


DROP TABLE IF EXISTS tpt_objects  CASCADE; 

CREATE TABLE tpt_objects ( 
   id           integer NOT NULL AUTO_INCREMENT PRIMARY KEY, 
   name         varchar(1000), 
   type         varchar(200) NOT NULL, 
   table_name   varchar(200)  NOT NULL, 

   created_at    datetime, 
   updated_at    timestamp 
 ); 

-- CREATE TABLE tpt_managed_objects ( 
--   id           integer PRIMARY KEY, 
--   display_name varchar(200), 
--   description  varchar(2000), 
--   created_at    datetime, 
--   updated_at    timestamp, 

--   FOREIGN KEY(id) REFERENCES tpt_objects(id)  on delete cascade 
-- );
`

const test_text1 = `
-- DROP TABLE if exists tpt_migrate_logs;

create table if not exists tpt_migrate_logs (
  id                 INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  name               varchar(250)  NOT NULL,
  logtext            TEXT, 
  executed           boolean DEFAULT '0',
  created_at         timestamp
);

DROP PROCEDURE IF EXISTS tpt_delete_mo_trigger;

-- +goose StatementBegin
-- 需要注意的是当前 mysql 不支持在 PREPARE 语句中执行 CREATE TRIGGER 和 DROP TRIGGER
-- 请见 https://dev.mysql.com/worklog/task/?id=2871
-- 所以下列 tpt_delete_mo_trigger 不可被调用，调用会返回 SQL 错误 [1295] [HY000]: This command is not supported in the prepared statement protocol yet
-- 为兼容性和可读性，我在程序中找到它，替换它
CREATE PROCEDURE tpt_delete_mo_trigger(tablename text) -- $1 is tablename
BEGIN
    SET @SQLSTR = CONCAT("DROP TRIGGER  IF EXISTS ", tablename, "_trigger_on_updated");
    set @LOGNAME = CONCAT('del ', tablename, '_trigger_on_updated');
    insert into tpt_migrate_logs (name, logtext, created_at) values(@LOGNAME, @SQLSTR, NOW());
    -- PREPARE u_trstmt FROM @SQLSTR;
    -- EXECUTE u_trstmt;

    SET @SQLSTR = CONCAT('DROP TRIGGER  IF EXISTS ', tablename, '_trigger_on_deleted;');
    set @LOGNAME = CONCAT('del ', tablename, '_trigger_on_deleted');
    insert into tpt_migrate_logs (name, logtext, created_at) values(@LOGNAME, @SQLSTR, NOW());
    -- PREPARE d_trstmt FROM @SQLSTR;
    -- EXECUTE d_trstmt;

    SET @SQLSTR = CONCAT('DROP TRIGGER  IF EXISTS ', tablename, '_trigger_on_creating;');
    set @LOGNAME = CONCAT('del ', tablename, '_trigger_on_creating');
    insert into tpt_migrate_logs (name, logtext, created_at) values(@LOGNAME, @SQLSTR, NOW());
    -- PREPARE c_trstmt FROM @SQLSTR;
    -- EXECUTE c_trstmt;
END;
-- +goose StatementEnd

delimiter ;

DROP PROCEDURE IF EXISTS tpt_create_mo_trigger;

-- +goose StatementBegin
-- 需要注意的是当前 mysql 不支持在 PREPARE 语句中执行 CREATE TRIGGER 和 DROP TRIGGER
-- 请见 https://dev.mysql.com/worklog/task/?id=2871
-- 所以下列 tpt_delete_mo_trigger 不可被调用，调用会返回 SQL 错误 [1295] [HY000]: This command is not supported in the prepared statement protocol yet
-- 为兼容性和可读性，我在程序中找到它，替换它
CREATE PROCEDURE tpt_create_mo_trigger(tablename TEXT, objectType TEXT, fieldName TEXT) -- $1 is tablename, $2 is type
BEGIN
  SET @SQLSTR = CONCAT('CREATE TRIGGER ', tablename, '_trigger_on_creating
      BEFORE INSERT ON ', tablename, '
      FOR EACH ROW
      BEGIN
       IF NEW.created_at IS NULL THEN
           SET NEW.created_at = NOW();
        END IF;
        IF NEW.updated_at IS NULL THEN
           SET NEW.updated_at = NOW();
        END IF;
        
        INSERT INTO tpt_objects(name, type, table_name, created_at, updated_at)
          VALUES (NEW.display_name, ', objectType, ', ''', tablename, ''', NEW.created_at, NEW.updated_at);

        SET NEW.id =  LAST_INSERT_ID();
      END;');
  set @LOGNAME = CONCAT('create ', tablename, '_trigger_on_creating');
  insert into tpt_migrate_logs (name, logtext, created_at) values(@LOGNAME, @SQLSTR, NOW());
  -- PREPARE c_trstmt FROM @SQLSTR;
  -- EXECUTE c_trstmt;

  SET @SQLSTR = CONCAT('CREATE TRIGGER ', tablename, '_trigger_on_updated
      BEFORE UPDATE ON ', tablename, '
      FOR EACH ROW
      BEGIN
        IF NEW.updated_at IS NULL THEN
           NEW.updated_at = NOW();
        END IF;
       
        UPDATE  tpt_objects set name=NEW.', fieldName, ', updated_at=NEW.updated_at WHERE id = NEW.id;
      END;');
  set @LOGNAME = CONCAT('create ', tablename, '_trigger_on_updated');
  insert into tpt_migrate_logs (name, logtext, created_at) values(@LOGNAME, @SQLSTR, NOW());
  -- PREPARE u_trstmt FROM @SQLSTR;
  -- EXECUTE u_trstmt;

  SET @SQLSTR = CONCAT('CREATE TRIGGER ', tablename, '_trigger_on_deleted
      AFTER DELETE ON ', tablename, '
      FOR EACH ROW
      BEGIN
        DELETE FROM tpt_objects cascade WHERE id = OLD.id;
      END;');
  set @LOGNAME = CONCAT('del ', tablename, '_trigger_on_deleted');
    insert into tpt_migrate_logs (name, logtext, created_at) values(@LOGNAME, @SQLSTR, NOW());
  -- PREPARE d_trstmt FROM @SQLSTR;
  -- EXECUTE d_trstmt;
END;
-- +goose StatementEnd

delimiter ;

DROP TABLE IF EXISTS tpt_domains CASCADE;

CREATE TABLE tpt_domains (
  id                 INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  name               varchar(50) NOT NULL,
  description        varchar(500),
  uuid               varchar(50) NOT NULL,
  created_at         datetime,
  updated_at         timestamp,

  unique(uuid)
);



DROP TABLE IF EXISTS tpt_engine_nodes  CASCADE;

CREATE TABLE tpt_engine_nodes (
  id                     integer NOT NULL AUTO_INCREMENT PRIMARY KEY,

  name                   varchar(250) NOT NULL,
  description            varchar(2000),
  fields                 json,

  created_at    datetime,
  updated_at    timestamp,

  UNIQUE(name)
);

INSERT INTO tpt_engine_nodes(name, description, created_at, updated_at) VALUES('default', '默认的采集引擎', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);


DROP TABLE IF EXISTS tpt_objects  CASCADE;

CREATE TABLE tpt_objects (
  id           integer NOT NULL AUTO_INCREMENT PRIMARY KEY,
  name         varchar(1000),
  type         varchar(200) NOT NULL,
  table_name   varchar(200)  NOT NULL,

  created_at    datetime,
  updated_at    timestamp
);

-- CREATE TABLE tpt_managed_objects (
--   id           integer PRIMARY KEY,
--   display_name varchar(200),
--   description  varchar(2000),
--   created_at    datetime,
--   updated_at    timestamp,

--   FOREIGN KEY(id) REFERENCES tpt_objects(id)  on delete cascade
-- );
`
