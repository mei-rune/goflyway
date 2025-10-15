package goflyway

import (
	"archive/zip"
	"io/fs"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestIsFlywayFilename 测试文件名识别
func TestIsFlywayFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"Valid filename", "V1__init.sql", true},
		{"Valid with numbers", "V1.2.34__create_table.sql", true},
		{"Missing V prefix", "1__test.sql", false},
		{"Missing __ separator", "V1_test.sql", false},
		{"Wrong extension", "V1__test.txt", false},
		{"Empty filename", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFlywayFilename(tt.filename); got != tt.expected {
				t.Errorf("isFlywayFilename(%q) = %v, want %v", tt.filename, got, tt.expected)
			}
		})
	}
}

// TestParseFlywayVersion 测试版本号解析
func TestParseFlywayVersion(t *testing.T) {
	tests := []struct {
		name        string
		versionStr  string
		expectMajor int
		expectMinor int
		expectPatch int
		expectErr   bool
	}{
		{"Valid version", "1.2.345", 1, 2, 345, false},
		{"Max values", "12.31.999999", 12, 31, 999999, false},

		{"Non-Invalid format", "1", 1, 1, 0, false},
		{"Non-Invalid format", "1.1", 1, 1, 0, false},

		// {"Invalid format", "1.2", 0, 0, 0, true},
		{"Non-numbers", "a.b.c", 0, 0, 0, true},
		{"Major too big", "13.1.1", 0, 0, 0, true},
		{"Minor too big", "1.32.1", 0, 0, 0, true},
		{"Patch too big", "1.1.1000000", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, patch, err := parseFlywayVersion(tt.versionStr)

			if (err != nil) != tt.expectErr {
				t.Errorf("parseFlywayVersion() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if !tt.expectErr {
				if major != tt.expectMajor || minor != tt.expectMinor || patch != tt.expectPatch {
					t.Errorf("parseFlywayVersion() = %d, %d, %d, want %d, %d, %d",
						major, minor, patch, tt.expectMajor, tt.expectMinor, tt.expectPatch)
				}
			}
		})
	}
}

// TestConvertToGooseTimestamp 测试时间戳转换
func TestConvertToGooseTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		baseYear  string
		expected  string
		expectErr bool
	}{
		{"Normal case", "1.2.345", "2000", "20000102000345", false},
		{"Max values", "12.31.9999", "2000", "20001231009999", false},
		{"Different base year", "1.1.1", "2020", "20200101000001", false},
		{"Invalid version", "a.b.c", "2000", "", true},
		// {"Invalid base year", "1.1.1", "invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToGooseTimestamp(tt.version, tt.baseYear)

			if (err != nil) != tt.expectErr {
				t.Errorf("convertToGooseTimestamp() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if !tt.expectErr && result != tt.expected {
				t.Errorf("convertToGooseTimestamp() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestConvertToGooseFilename 测试文件名转换
func TestConvertToGooseFilename(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		baseYear  string
		expected  string
		expectErr bool
	}{
		{"Simple case", "V1__init.sql", "2000", "20000101000000_init.sql", false},
		{"Simple case", "V1.1__init.sql", "2000", "20000101000000_init.sql", false},
		{"Complex name", "V1.2.34__create_users_table.sql", "2000", "20000102000034_create_users_table.sql", false},
		{"Invalid filename", "invalid.txt", "2000", "", true},
		{"Invalid version", "Va.b.c__test.sql", "2000", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToGooseFilename(tt.filename, tt.baseYear)

			if (err != nil) != tt.expectErr {
				t.Errorf("convertToGooseFilename() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if !tt.expectErr && result != tt.expected {
				t.Errorf("convertToGooseFilename() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestProcessFS 测试文件系统处理
func TestProcessFS(t *testing.T) {
	go http.ListenAndServe(":", nil)
	// 创建测试文件系统
	testFS := os.DirFS("testdata")

	// 创建临时输出目录
	tempDir, err := os.MkdirTemp("", "goose_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 测试处理文件系统
	err = processFS(testFS, tempDir, "2000")
	if err != nil {
		t.Errorf("processFS() error = %v", err)
	}

	// 检查输出文件
	expectedFiles := []string{
		"20000101000000_first_migration.sql",
		"20000102000003_second_migration.sql",
	}

	ok := true
	for _, filename := range expectedFiles {
		path := filepath.Join(tempDir, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s was not created", filename)

			ok = false
		}
	}

	if !ok {
		fis, err := os.ReadDir(tempDir)
		if err != nil {
			t.Errorf("processFS() error = %v", err)
		}
		for _, fi := range fis {
			t.Log(fi.Name())
		}
	}
}

// TestGetInputFS 测试获取输入文件系统
func TestGetInputFS(t *testing.T) {
	// 测试目录文件系统
	dirFS, closer, err := getInputFS(nil, "testdata")
	if err != nil {
		t.Errorf("getInputFS() with dir error = %v", err)
	}
	if closer != nil {
		closer.Close()
	}
	if _, ok := dirFS.(fs.FS); !ok {
		t.Error("Expected dirFS to implement fs.FS")
	}

	// 创建测试JAR文件
	jarPath := filepath.Join(t.TempDir(), "test.jar")
	createTestJar(jarPath)

	// 测试JAR文件系统
	jarFS, closer, err := getInputFS(nil, jarPath)
	if err != nil {
		t.Errorf("getInputFS() with jar error = %v", err)
	}
	if closer == nil {
		t.Error("Expected jarFS to have a closer")
	} else {
		closer.Close()
	}
	if _, ok := jarFS.(fs.FS); !ok {
		t.Error("Expected jarFS to implement fs.FS")
	}
}

// createTestJar 创建一个测试用的JAR文件
func createTestJar(path string) {
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	// 添加一个Flyway迁移文件
	writer, err := zipWriter.Create("V1__test.sql")
	if err != nil {
		panic(err)
	}
	writer.Write([]byte("CREATE TABLE test (id INT);"))
}

// TestConvertFlywayToGoose 测试完整转换流程
func TestConvertFile(t *testing.T) {
	// 创建临时输出目录
	tempDir := t.TempDir()

	// 测试目录转换
	outputDir, err := Convert("testdata", tempDir, "2000")
	if err != nil {
		t.Errorf("ConvertFlywayToGoose() error = %v", err)
	}

	// 检查输出目录
	files, err := os.ReadDir(outputDir)
	if err != nil {
		t.Errorf("Failed to read output dir: %v", err)
	}

	if len(files) == 0 {
		t.Error("No files were converted")
	}
}

// TestConvertAndMigrate 测试转换并迁移(模拟)
func TestConvertAndMigrate(t *testing.T) {
	// 使用模拟数据库连接
	cfg := &Config{
		InputPath:    "testdata",
		OutputDir:    t.TempDir(),
		BaseYear:     "2000",
		DBConnString: "file:test.db?mode=memory&cache=shared",
		DBDriver:     "sqlite3",
	}

	err := ConvertAndMigrate(cfg)
	if err != nil {
		t.Errorf("ConvertAndMigrate() error = %v", err)
	}
}
