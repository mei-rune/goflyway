package goflyway

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pressly/goose/v3"
)

type Config struct {
	BaseYear     string
	InputPath    string
	OutputDir    string
	DBDriver     string
	DBConnString string
}

func Convert(inputPath, outputDir, baseYear string) (string, error) {
	inputFS, closer, err := getInputFS(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to initialize input filesystem: %w", err)
	}
	if closer != nil {
		defer closer.Close()
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	err = processFS(inputFS, outputDir, baseYear)
	return outputDir, err
}

func migrateWithGoose(migrationsDir, driver, connString string) error {
	db, err := goose.OpenDBWithDriver(driver, connString)
	if err != nil {
		return fmt.Errorf("failed to open DB: %w", err)
	}
	defer db.Close()

	if err := goose.SetDialect(driver); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	return goose.Up(db, migrationsDir)
}

func ConvertAndMigrate(cfg *Config) error {
	var migrationsDir string
	var err error

	useTempDir := false
	if cfg.OutputDir == "" {
		// 创建临时目录
		cfg.OutputDir, err = os.MkdirTemp("", "flyway2goose_")
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}
		useTempDir = true
	}

	migrationsDir, err = Convert(cfg.InputPath, cfg.OutputDir, cfg.BaseYear)
	if err != nil {
		return err
	}

	err = migrateWithGoose(migrationsDir, cfg.DBDriver, cfg.DBConnString)

	if useTempDir {
		// 如果使用了临时目录，迁移完成后删除
		os.RemoveAll(cfg.OutputDir)
	}

	return err
}

func RunMain() {
	command, cfg, err := parseArgs()
	if err != nil {
		fmt.Printf("参数错误: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}

	var executeErr error
	switch command {
	case "convert":
		if cfg.InputPath == "" || cfg.OutputDir == "" {
			fmt.Println("convert 命令需要 input 和 output 参数")
			flag.Usage()
			os.Exit(1)
		}
		_, executeErr = Convert(cfg.InputPath, cfg.OutputDir, cfg.BaseYear)
	case "run":
		if cfg.InputPath == "" || cfg.DBDriver == "" || cfg.DBConnString == "" {
			fmt.Println("run 命令需要 input，db_driver 和 db_url 参数")
			flag.Usage()
			os.Exit(1)
		}
		executeErr = ConvertAndMigrate(cfg)
	default:
		fmt.Printf("未知命令: %s\n", command)
		os.Exit(1)
	}

	if executeErr != nil {
		fmt.Printf("执行失败: %v\n", executeErr)
		os.Exit(1)
	}
}

func parseArgs() (string, *Config, error) {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	cfg := &Config{}

	switch command {
	case "convert":
		convertCmd := flag.NewFlagSet("convert", flag.ExitOnError)
		convertCmd.StringVar(&cfg.InputPath, "input", "", "输入路径(JAR文件或目录)(必需)")
		convertCmd.StringVar(&cfg.OutputDir, "output", "", "输出目录(必需)")
		convertCmd.StringVar(&cfg.BaseYear, "year", "2000", "基础年份(用于版本转换)")
		if err := convertCmd.Parse(os.Args[2:]); err != nil {
			return command, nil, err
		}

	case "run":
		runCmd := flag.NewFlagSet("run", flag.ExitOnError)
		runCmd.StringVar(&cfg.InputPath, "input", "", "输入路径(JAR文件或目录)(必需)")
		runCmd.StringVar(&cfg.OutputDir, "output", "", "输出目录(可选，为空时使用临时目录)")
		runCmd.StringVar(&cfg.BaseYear, "year", "2000", "基础年份(用于版本转换)")
		runCmd.StringVar(&cfg.DBDriver, "db_driver", "postgres", "数据库驱动(postgres/mysql/sqlite3等)")
		runCmd.StringVar(&cfg.DBConnString, "db_url", "", "数据库连接字符串(必需)")
		if err := runCmd.Parse(os.Args[2:]); err != nil {
			return command, nil, err
		}
	default:
		printUsage()
		os.Exit(1)
	}

	return command, cfg, nil
}

func printUsage() {
	fmt.Println("使用方法:")
	fmt.Println("  convert - 仅转换迁移脚本")
	fmt.Println("    flyway convert -input <path> -output <dir> [-year <year>]")
	fmt.Println("    参数:")
	fmt.Println("      -year:   可选，基础年份(默认2000)")
	fmt.Println("      -input:  必需，输入路径(JAR文件或目录)")
	fmt.Println("      -output: 必需，输出目录")

	fmt.Println("\n  run - 转换并执行迁移")
	fmt.Println("    flyway run -input <path> [-db_driver <name>] -db_url <conn> [-output <dir>] [-year <year>]")
	fmt.Println("    参数:")
	fmt.Println("      -year:   可选，基础年份(默认2000)")
	fmt.Println("      -input:  必需，输入路径(JAR文件或目录)")
	fmt.Println("      -output: 可选，输出目录(为空时使用临时目录)")
	fmt.Println("      -db_driver:  可选，数据库驱动(默认postgres)")
	fmt.Println("      -db_url:     必需，数据库连接字符串")
}

// getInputFS 根据输入路径返回适当的文件系统实现
func getInputFS(inputPath string) (fs.FS, io.Closer, error) {
	if strings.HasSuffix(strings.ToLower(inputPath), ".jar") {
		zipFS, err := zip.OpenReader(inputPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open JAR file: %w", err)
		}
		return zipFS, zipFS, nil
	}
	return os.DirFS(inputPath), nil, nil
}

// processFS 处理文件系统中的 Flyway 迁移文件
func processFS(fsys fs.FS, outputDir string, baseYear string) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !isFlywayFilename(path) {
			return nil
		}

		file, err := fsys.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", path, err)
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		gooseName, err := convertToGooseFilename(path, baseYear)
		if err != nil {
			return fmt.Errorf("failed to convert filename %s: %w", path, err)
		}

		outputPath := filepath.Join(outputDir, gooseName)
		if err := os.WriteFile(outputPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", outputPath, err)
		}

		fmt.Printf("Converted: %s -> %s\n", path, gooseName)
		return nil
	})
}

// isFlywayFilename 检查文件名是否符合 Flyway 格式
func isFlywayFilename(name string) bool {
	name = filepath.Base(name)
	return strings.HasPrefix(name, "V") &&
		strings.Contains(name, "__") &&
		strings.HasSuffix(name, ".sql")
}

// convertToGooseFilename 将 Flyway 文件名转换为 Goose 格式
func convertToGooseFilename(flywayName string, baseYear string) (string, error) {
	base := strings.TrimSuffix(filepath.Base(flywayName), ".sql")
	parts := strings.SplitN(base, "__", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid Flyway filename format")
	}

	versionStr := strings.TrimPrefix(parts[0], "V")
	timestamp, err := convertToGooseTimestamp(versionStr, baseYear)
	if err != nil {
		return "", err
	}

	description := strings.Map(func(r rune) rune {
		switch {
		case r == '_' || r == '-':
			return '_'
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			return r
		default:
			return -1
		}
	}, parts[1])

	return fmt.Sprintf("%s_%s.sql", timestamp, description), nil
}

// convertToGooseTimestamp 将 Flyway 版本号转换为 Goose 时间戳
func convertToGooseTimestamp(versionStr string, baseYear string) (string, error) {
	major, minor, patch, err := parseFlywayVersion(versionStr)
	if err != nil {
		return "", err
	}

	timestamp := fmt.Sprintf("%02d%02d%06d",
		major,
		minor,
		patch)

	if len(timestamp) != 10 {
		return "", fmt.Errorf("invalid timestamp length")
	}
	return baseYear + timestamp, nil
}

// parseFlywayVersion 解析 Flyway 版本号
func parseFlywayVersion(versionStr string) (major, minor, patch int, err error) {
	parts := strings.Split(versionStr, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return 0, 0, 0, fmt.Errorf("version format should be Vx.x.xxx")
	}

	major, err = strconv.Atoi(parts[0])
	if err != nil || major < 1 || major > 12 {
		return 0, 0, 0, fmt.Errorf("major version must be 1-12")
	}
	if len(parts) == 1 {
		return major, 1, 0, nil
	}

	minor, err = strconv.Atoi(parts[1])
	if err != nil || minor < 1 || minor > 31 {
		return 0, 0, 0, fmt.Errorf("minor version must be 1-31")
	}
	if len(parts) == 2 {
		return major, minor, 0, nil
	}

	patch, err = strconv.Atoi(parts[2])
	if err != nil || patch < 1 || patch > 999999 {
		return 0, 0, 0, fmt.Errorf("patch version must be 1-999999")
	}
	return major, minor, patch, nil
}
