package main

import (
	"github.com/mei-rune/goflyway"

	_ "gitee.com/chunanyong/dm"                       // 达梦
	_ "gitee.com/opengauss/openGauss-connector-go-pq" // openGauss
	_ "gitee.com/runner.mei/gokb"                     // 人大金仓
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/sijms/go-ora/v2"
	_ "github.com/ziutek/mymysql/godrv"
)

func main() {
	goflyway.RunMain()
}
