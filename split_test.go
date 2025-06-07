package goflyway

import (
	"reflect"
	"strings"
	"testing"
)

func TestMultipleStatements(t *testing.T) {
	input := "SELECT 1; SELECT 2;"
	expected := []string{"SELECT 1;", " SELECT 2;"}
	result, _ := Split(strings.NewReader(input))
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected: %v, Got: %v", expected, result)
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
