# goflyway


When adapting Flyway to new or legacy databases, it can be quite troublesome. It throws errors like "ERROR: Unsupported Database: xxx", when in reality they're just SQL scripts that could be executed sequentially. It seems excessive to modify Flyway itself just to make it work.

For running Flyway scripts in Golang, my approach is simple: convert the Flyway scripts into Goose scripts, then execute them using Goose.

Thanks to DeepSeek, I used it to generate all the code and tests, and then completed the project with some minor tweaks.


flyway 在适配新数据库或老数据库时很麻烦，它会报 ERROR: Unsupported Database: xxx, 其实只是一些 sql 脚本而已，你只要按顺序执行就可以了。 结果你为了用 flyway 还要修改它就太夸张了。

在 golang 中运行 flyway 脚本, 我的方法很简单，将 flyway 脚本转成 goose 脚本，然后再用 goose 运行它。


感谢 deepseek， 我用它生成了所有代码和测试， 然后稍作修改就完成了它！