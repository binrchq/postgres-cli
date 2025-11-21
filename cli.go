package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// Terminal 终端接口，用于输入输出
type Terminal interface {
	io.Reader
	io.Writer
}

// Config PostgreSQL 连接配置
type Config struct {
	Host            string
	Port            int
	Username        string
	Password        string
	Database        string
	SSLMode         string        // SSL模式：disable/require/verify-ca/verify-full，默认 disable
	ConnectTimeout  time.Duration // 连接超时，默认 10s
	StatementTimeout time.Duration // 语句超时，默认 0（无限制）
	MaxOpenConns    int           // 最大连接数，默认 10
	MaxIdleConns    int           // 最大空闲连接数，默认 5
	ConnMaxLifetime time.Duration // 连接最大生命周期，默认 1h
	ApplicationName string        // 应用名称
	SearchPath      string        // 搜索路径
	TimeZone        string        // 时区
	CustomParams    string        // 自定义参数，如 "param1=value1&param2=value2"
}

// CLI PostgreSQL 交互式命令行客户端
type CLI struct {
	term          Terminal
	config        *Config
	db            *sql.DB
	reader        *Reader
	serverInfo    ServerInfo
	expandedMode  bool // \x 扩展显示模式
	timingEnabled bool // \timing 计时
	maxRows       int  // 最大显示行数
	inTransaction bool // 是否在事务中
	database      string
}

// ServerInfo PostgreSQL 服务器信息
type ServerInfo struct {
	Version       string
	ServerEncoding string
	ClientEncoding string
	ConnectionID  int
}

// NewCLI 创建新的 PostgreSQL CLI 实例（兼容旧接口）
func NewCLI(term Terminal, host string, port int, username, password, database string) *CLI {
	config := &Config{
		Host:            host,
		Port:            port,
		Username:        username,
		Password:        password,
		Database:        database,
		SSLMode:         "disable",
		ConnectTimeout:  10 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		ApplicationName: "psql",
	}
	return NewCLIWithConfig(term, config)
}

// NewCLIWithConfig 使用配置创建 PostgreSQL CLI 实例
func NewCLIWithConfig(term Terminal, config *Config) *CLI {
	// 设置默认值
	if config.SSLMode == "" {
		config.SSLMode = "disable"
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 10 * time.Second
	}
	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 10
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 5
	}
	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = time.Hour
	}
	if config.ApplicationName == "" {
		config.ApplicationName = "psql"
	}

	return &CLI{
		term:     term,
		config:   config,
		database: config.Database,
		reader:   NewReader(term),
		maxRows:  1000,
		timingEnabled: false,
	}
}

// Connect 连接到 PostgreSQL 数据库
func (c *CLI) Connect() error {
	// 构建 DSN
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		c.config.Host,
		c.config.Port,
		c.config.Username,
		c.config.Password,
		c.config.Database,
		c.config.SSLMode,
		int(c.config.ConnectTimeout.Seconds()),
	)

	// 添加可选参数
	if c.config.ApplicationName != "" {
		dsn += fmt.Sprintf(" application_name=%s", c.config.ApplicationName)
	}
	if c.config.SearchPath != "" {
		dsn += fmt.Sprintf(" search_path=%s", c.config.SearchPath)
	}
	if c.config.TimeZone != "" {
		dsn += fmt.Sprintf(" timezone=%s", c.config.TimeZone)
	}
	if c.config.StatementTimeout > 0 {
		dsn += fmt.Sprintf(" statement_timeout=%d", int(c.config.StatementTimeout.Milliseconds()))
	}
	if c.config.CustomParams != "" {
		dsn += " " + c.config.CustomParams
	}

	var err error
	c.db, err = sql.Open("postgres", dsn)
	if err != nil {
		return err
	}

	// 设置连接池参数
	c.db.SetMaxOpenConns(c.config.MaxOpenConns)
	c.db.SetMaxIdleConns(c.config.MaxIdleConns)
	c.db.SetConnMaxLifetime(c.config.ConnMaxLifetime)

	if err := c.db.Ping(); err != nil {
		c.db.Close()
		return err
	}

	// 获取服务器信息
	c.fetchServerInfo()

	// 显示欢迎信息
	c.showWelcome()

	return nil
}

// fetchServerInfo 获取服务器信息
func (c *CLI) fetchServerInfo() {
	var version string
	c.db.QueryRow("SELECT version()").Scan(&version)
	c.serverInfo.Version = version

	var serverEncoding, clientEncoding string
	c.db.QueryRow("SHOW server_encoding").Scan(&serverEncoding)
	c.db.QueryRow("SHOW client_encoding").Scan(&clientEncoding)
	c.serverInfo.ServerEncoding = serverEncoding
	c.serverInfo.ClientEncoding = clientEncoding

	var connID int
	c.db.QueryRow("SELECT pg_backend_pid()").Scan(&connID)
	c.serverInfo.ConnectionID = connID
}

// showWelcome 显示欢迎信息
func (c *CLI) showWelcome() {
	fmt.Fprintf(c.term, "psql (%s)\n", extractVersionNumber(c.serverInfo.Version))
	fmt.Fprintf(c.term, "Type \"help\" for help.\n\n")
}

// extractVersionNumber 从版本字符串中提取版本号
func extractVersionNumber(version string) string {
	parts := strings.Fields(version)
	if len(parts) >= 2 {
		return parts[1]
	}
	return version
}

// Start 启动交互式命令行
func (c *CLI) Start() error {
	for {
		// 设置提示符
		prompt := c.getPrompt()
		c.reader.SetPrompt(prompt)
		
		// 支持多行 SQL（以分号结束）
		sqlStr := c.readMultiLine()
		if sqlStr == "" {
			continue
		}

		sqlStr = strings.TrimSpace(sqlStr)
		
		// 处理 psql 特殊命令（不需要分号）
		if c.handlePsqlCommand(sqlStr) {
			if strings.ToLower(sqlStr) == "exit" || strings.ToLower(sqlStr) == "quit" || 
			   sqlStr == "\\q" {
				return nil
			}
			continue
		}

		// 执行 SQL
		c.executeSQL(sqlStr)
	}
}

// getPrompt 获取提示符
func (c *CLI) getPrompt() string {
	if c.inTransaction {
		return fmt.Sprintf("%s*=> ", c.database)
	}
	return fmt.Sprintf("%s=> ", c.database)
}

// readMultiLine 读取多行 SQL（以分号结束）
func (c *CLI) readMultiLine() string {
	var lines []string
	
	for {
		line, err := c.reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				return ""
			}
			return ""
		}
		
		trimmed := strings.TrimSpace(line)
		
		// 空行继续等待输入
		if trimmed == "" && len(lines) == 0 {
			return ""
		}
		
		// 如果是第一行，检查是否是特殊命令（不需要分号）
		if len(lines) == 0 {
			// 如果是 psql 命令（以反斜杠开头），直接返回
			if strings.HasPrefix(trimmed, "\\") {
				return trimmed
			}
			// 检查其他特殊命令（exit, quit, help）
			cmdLower := strings.ToLower(trimmed)
			if cmdLower == "exit" || cmdLower == "quit" || cmdLower == "help" {
				return trimmed
			}
		}
		
		// 如果已经有输入且当前行是反斜杠命令，追加这行
		if len(lines) > 0 && strings.HasPrefix(trimmed, "\\") {
			lines = append(lines, line)
			continue
		}
		
		lines = append(lines, line)
		
		// 检查是否以分号结束
		if strings.HasSuffix(trimmed, ";") {
			break
		}
		
		// 设置多行提示符
		c.reader.SetPrompt(fmt.Sprintf("%s-> ", c.database))
	}
	
	result := strings.Join(lines, "\n")
	return result
}

// executeSQL 执行 SQL 语句
func (c *CLI) executeSQL(sqlStr string) {
	startTime := time.Now()
	
	// 移除末尾的分号
	sqlStr = strings.TrimSuffix(strings.TrimSpace(sqlStr), ";")
	sqlStr = strings.TrimSpace(sqlStr)
	
	if sqlStr == "" {
		return
	}
	
	// 检查是否是事务命令
	upperSQL := strings.ToUpper(sqlStr)
	if upperSQL == "BEGIN" || upperSQL == "START TRANSACTION" {
		c.inTransaction = true
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := c.db.ExecContext(ctx, "BEGIN")
		if err != nil {
			fmt.Fprintf(c.term, "ERROR: %v\n", err)
			return
		}
		fmt.Fprintf(c.term, "BEGIN\n")
		if c.timingEnabled {
			fmt.Fprintf(c.term, "Time: %.3f ms\n", time.Since(startTime).Seconds()*1000)
		}
		return
	}
	if upperSQL == "COMMIT" {
		c.inTransaction = false
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := c.db.ExecContext(ctx, "COMMIT")
		if err != nil {
			fmt.Fprintf(c.term, "ERROR: %v\n", err)
			return
		}
		fmt.Fprintf(c.term, "COMMIT\n")
		if c.timingEnabled {
			fmt.Fprintf(c.term, "Time: %.3f ms\n", time.Since(startTime).Seconds()*1000)
		}
		return
	}
	if upperSQL == "ROLLBACK" {
		c.inTransaction = false
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := c.db.ExecContext(ctx, "ROLLBACK")
		if err != nil {
			fmt.Fprintf(c.term, "ERROR: %v\n", err)
			return
		}
		fmt.Fprintf(c.term, "ROLLBACK\n")
		if c.timingEnabled {
			fmt.Fprintf(c.term, "Time: %.3f ms\n", time.Since(startTime).Seconds()*1000)
		}
		return
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	if isQuery(sqlStr) {
		c.executeQuery(ctx, sqlStr, startTime)
	} else {
		c.executeCommand(ctx, sqlStr, startTime)
	}
}

// handlePsqlCommand 处理 psql 特殊命令
func (c *CLI) handlePsqlCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	cmdLower := strings.ToLower(cmd)
	
	// Exit commands
	if cmd == "\\q" || cmdLower == "exit" || cmdLower == "quit" {
		fmt.Fprintf(c.term, "\n")
		return true
	}
	
	// Help
	if cmd == "\\?" || cmdLower == "help" {
		c.showHelp()
		return true
	}
	
	// SQL help
	if cmd == "\\h" || strings.HasPrefix(cmd, "\\h ") {
		c.showSQLHelp(cmd)
		return true
	}
	
	// List databases
	if cmd == "\\l" || cmd == "\\list" {
		c.executeSQL("SELECT datname AS \"Name\", pg_catalog.pg_get_userbyid(datdba) AS \"Owner\", pg_catalog.pg_encoding_to_char(encoding) AS \"Encoding\" FROM pg_catalog.pg_database ORDER BY datname")
		return true
	}
	
	// Connect to database
	if strings.HasPrefix(cmd, "\\c ") || strings.HasPrefix(cmd, "\\connect ") {
		parts := strings.Fields(cmd)
		if len(parts) >= 2 {
			c.connectToDatabase(parts[1])
		} else {
			fmt.Fprintf(c.term, "ERROR: database name required\n")
		}
		return true
	}
	
	// List tables
	if cmd == "\\dt" || cmd == "\\dt+" {
		c.executeSQL("SELECT schemaname AS \"Schema\", tablename AS \"Name\", tableowner AS \"Owner\" FROM pg_catalog.pg_tables WHERE schemaname NOT IN ('pg_catalog', 'information_schema') ORDER BY schemaname, tablename")
		return true
	}
	
	// List schemas
	if cmd == "\\dn" || cmd == "\\dn+" {
		c.executeSQL("SELECT nspname AS \"Name\", pg_catalog.pg_get_userbyid(nspowner) AS \"Owner\" FROM pg_catalog.pg_namespace WHERE nspname !~ '^pg_' AND nspname <> 'information_schema' ORDER BY nspname")
		return true
	}
	
	// Describe table
	if strings.HasPrefix(cmd, "\\d ") {
		tableName := strings.TrimSpace(cmd[3:])
		c.describeTable(tableName)
		return true
	}
	
	// List views
	if cmd == "\\dv" || cmd == "\\dv+" {
		c.executeSQL("SELECT schemaname AS \"Schema\", viewname AS \"Name\", viewowner AS \"Owner\" FROM pg_catalog.pg_views WHERE schemaname NOT IN ('pg_catalog', 'information_schema') ORDER BY schemaname, viewname")
		return true
	}
	
	// List indexes
	if cmd == "\\di" || cmd == "\\di+" {
		c.executeSQL("SELECT schemaname AS \"Schema\", indexname AS \"Name\", tablename AS \"Table\" FROM pg_catalog.pg_indexes WHERE schemaname NOT IN ('pg_catalog', 'information_schema') ORDER BY schemaname, indexname")
		return true
	}
	
	// List sequences
	if cmd == "\\ds" || cmd == "\\ds+" {
		c.executeSQL("SELECT schemaname AS \"Schema\", sequencename AS \"Name\", sequenceowner AS \"Owner\" FROM pg_catalog.pg_sequences WHERE schemaname NOT IN ('pg_catalog', 'information_schema') ORDER BY schemaname, sequencename")
		return true
	}
	
	// List functions
	if cmd == "\\df" || cmd == "\\df+" {
		c.executeSQL("SELECT n.nspname AS \"Schema\", p.proname AS \"Name\", pg_catalog.pg_get_function_result(p.oid) AS \"Result data type\" FROM pg_catalog.pg_proc p LEFT JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname NOT IN ('pg_catalog', 'information_schema') ORDER BY n.nspname, p.proname")
		return true
	}
	
	// List users/roles
	if cmd == "\\du" || cmd == "\\du+" {
		c.executeSQL("SELECT rolname AS \"Role name\", rolsuper AS \"Superuser\", rolinherit AS \"Inherit\", rolcreaterole AS \"Create role\", rolcreatedb AS \"Create DB\" FROM pg_catalog.pg_roles ORDER BY rolname")
		return true
	}
	
	// Expanded display toggle
	if cmd == "\\x" {
		c.expandedMode = !c.expandedMode
		if c.expandedMode {
			fmt.Fprintf(c.term, "Expanded display is on.\n")
		} else {
			fmt.Fprintf(c.term, "Expanded display is off.\n")
		}
		return true
	}
	
	// Timing toggle
	if cmd == "\\timing" {
		c.timingEnabled = !c.timingEnabled
		if c.timingEnabled {
			fmt.Fprintf(c.term, "Timing is on.\n")
		} else {
			fmt.Fprintf(c.term, "Timing is off.\n")
		}
		return true
	}
	
	// Connection info
	if cmd == "\\conninfo" {
		c.showConnectionInfo()
		return true
	}
	
	// Current database
	if cmd == "\\d" {
		c.executeSQL("SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname = 'public' ORDER BY tablename")
		return true
	}
	
	return false
}

// connectToDatabase 连接到指定数据库
func (c *CLI) connectToDatabase(dbName string) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable connect_timeout=10",
		c.config.Host, c.config.Port, c.config.Username, c.config.Password, dbName)
	
	newDB, err := sql.Open("postgres", dsn)
	if err != nil {
		fmt.Fprintf(c.term, "ERROR: %v\n", err)
		return
	}
	
	if err := newDB.Ping(); err != nil {
		newDB.Close()
		fmt.Fprintf(c.term, "ERROR: database \"%s\" does not exist\n", dbName)
		return
	}
	
	// 关闭旧连接，使用新连接
	if c.db != nil {
		c.db.Close()
	}
	c.db = newDB
	c.database = dbName
	
	fmt.Fprintf(c.term, "You are now connected to database \"%s\" as user \"%s\".\n", dbName, c.config.Username)
}

// describeTable 描述表结构
func (c *CLI) describeTable(tableName string) {
	query := fmt.Sprintf(`
		SELECT 
			a.attname AS "Column",
			pg_catalog.format_type(a.atttypid, a.atttypmod) AS "Type",
			CASE WHEN a.attnotnull THEN 'not null' ELSE '' END AS "Modifiers"
		FROM pg_catalog.pg_attribute a
		WHERE a.attrelid = (
			SELECT c.oid FROM pg_catalog.pg_class c
			LEFT JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
			WHERE c.relname = '%s' AND n.nspname = 'public'
		) AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum
	`, tableName)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		fmt.Fprintf(c.term, "ERROR: %v\n", err)
		return
	}
	defer rows.Close()
	
	fmt.Fprintf(c.term, "Table \"%s\"\n", tableName)
	
	cols, _ := rows.Columns()
	colWidths := []int{10, 20, 15}
	
	c.printSeparator(colWidths)
	fmt.Fprintf(c.term, "| ")
	for i, col := range cols {
		fmt.Fprintf(c.term, "%-*s | ", colWidths[i], col)
	}
	fmt.Fprintf(c.term, "\n")
	c.printSeparator(colWidths)
	
	count := 0
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		valPtrs := make([]interface{}, len(cols))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}
		rows.Scan(valPtrs...)
		
		fmt.Fprintf(c.term, "| ")
		for i, v := range vals {
			var str string
			if v == nil {
				str = ""
			} else {
				str = fmt.Sprintf("%v", v)
			}
			fmt.Fprintf(c.term, "%-*s | ", colWidths[i], str)
		}
		fmt.Fprintf(c.term, "\n")
		count++
	}
	c.printSeparator(colWidths)
	fmt.Fprintf(c.term, "\n")
}

// showHelp 显示帮助信息
func (c *CLI) showHelp() {
	help := `
General
  \\?, help               show this help
  \\q, exit, quit         quit psql

Connection
  \\c [DBNAME]            connect to new database
  \\conninfo              display information about connection

Informational
  \\d [NAME]              describe table, view, sequence, or index
  \\dt[+]                 list tables
  \\dv[+]                 list views
  \\di[+]                 list indexes
  \\ds[+]                 list sequences
  \\df[+]                 list functions
  \\dn[+]                 list schemas
  \\du[+]                 list roles
  \\l, \\list             list databases

Formatting
  \\x                     toggle expanded output
  \\timing                toggle timing of commands

Transaction
  BEGIN                   start a transaction
  COMMIT                  commit current transaction
  ROLLBACK                rollback current transaction

Query Buffer
  \\h [NAME]              help on syntax of SQL commands

`
	fmt.Fprintf(c.term, help)
}

// showSQLHelp 显示 SQL 帮助
func (c *CLI) showSQLHelp(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 1 {
		fmt.Fprintf(c.term, "Available help:\n")
		fmt.Fprintf(c.term, "  SELECT, INSERT, UPDATE, DELETE\n")
		fmt.Fprintf(c.term, "  CREATE TABLE, DROP TABLE, ALTER TABLE\n")
		fmt.Fprintf(c.term, "  CREATE INDEX, DROP INDEX\n")
		fmt.Fprintf(c.term, "  BEGIN, COMMIT, ROLLBACK\n")
		fmt.Fprintf(c.term, "Use \\h <command> for help on specific command\n\n")
	} else {
		fmt.Fprintf(c.term, "No detailed help available for: %s\n", parts[1])
		fmt.Fprintf(c.term, "Please refer to PostgreSQL documentation.\n\n")
	}
}

// showConnectionInfo 显示连接信息
func (c *CLI) showConnectionInfo() {
	fmt.Fprintf(c.term, "You are connected to database \"%s\" as user \"%s\" via socket in \"%s\" at port \"%d\".\n",
		c.database, c.config.Username, c.config.Host, c.config.Port)
}

// Close 关闭数据库连接
func (c *CLI) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// executeQuery 执行查询语句
func (c *CLI) executeQuery(ctx context.Context, sqlStr string, startTime time.Time) {
	rows, err := c.db.QueryContext(ctx, sqlStr)
	if err != nil {
		c.printError(err)
		return
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	colTypes, _ := rows.ColumnTypes()
	
	if c.expandedMode {
		c.displayExpanded(rows, cols, startTime)
	} else {
		c.displayTable(rows, cols, colTypes, startTime)
	}
}

// displayTable 以表格形式显示结果
func (c *CLI) displayTable(rows *sql.Rows, cols []string, colTypes []*sql.ColumnType, startTime time.Time) {
	// 计算每列的最大宽度
	colWidths := make([]int, len(cols))
	for i, col := range cols {
		colWidths[i] = len(col)
		if colWidths[i] < 4 {
			colWidths[i] = 4
		}
		if colWidths[i] > 50 {
			colWidths[i] = 50
		}
	}
	
	// 收集所有行数据
	var allRows [][]string
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		valPtrs := make([]interface{}, len(cols))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}
		rows.Scan(valPtrs...)
		
		rowStrs := make([]string, len(vals))
		for i, v := range vals {
			if v == nil {
				rowStrs[i] = ""
			} else {
				switch val := v.(type) {
				case []byte:
					rowStrs[i] = string(val)
				case time.Time:
					rowStrs[i] = val.Format("2006-01-02 15:04:05")
				case bool:
					if val {
						rowStrs[i] = "t"
					} else {
						rowStrs[i] = "f"
					}
				default:
					rowStrs[i] = fmt.Sprintf("%v", v)
				}
			}
			
			// 更新列宽
			if len(rowStrs[i]) > colWidths[i] {
				if len(rowStrs[i]) > 50 {
					colWidths[i] = 50
					rowStrs[i] = rowStrs[i][:47] + "..."
				} else {
					colWidths[i] = len(rowStrs[i])
				}
			}
		}
		allRows = append(allRows, rowStrs)
		
		if len(allRows) >= c.maxRows {
			break
		}
	}
	
	// 打印表头
	fmt.Fprintf(c.term, " ")
	for i, col := range cols {
		fmt.Fprintf(c.term, "%-*s ", colWidths[i], col)
		if i < len(cols)-1 {
			fmt.Fprintf(c.term, "| ")
		}
	}
	fmt.Fprintf(c.term, "\n")
	
	// 打印分隔线
	for i, width := range colWidths {
		fmt.Fprintf(c.term, "%s", strings.Repeat("-", width+1))
		if i < len(colWidths)-1 {
			fmt.Fprintf(c.term, "+-")
		}
	}
	fmt.Fprintf(c.term, "\n")
	
	// 打印数据行
	for _, row := range allRows {
		fmt.Fprintf(c.term, " ")
		for i, val := range row {
			fmt.Fprintf(c.term, "%-*s ", colWidths[i], val)
			if i < len(row)-1 {
				fmt.Fprintf(c.term, "| ")
			}
		}
		fmt.Fprintf(c.term, "\n")
	}
	
	// 打印统计信息
	rowCount := len(allRows)
	if rowCount == 0 {
		fmt.Fprintf(c.term, "(0 rows)\n")
	} else if rowCount == 1 {
		fmt.Fprintf(c.term, "(1 row)\n")
	} else {
		fmt.Fprintf(c.term, "(%d rows)\n", rowCount)
	}
	
	if c.timingEnabled {
		elapsed := time.Since(startTime).Seconds() * 1000
		fmt.Fprintf(c.term, "Time: %.3f ms\n", elapsed)
	}
	fmt.Fprintf(c.term, "\n")
}

// printSeparator 打印表格分隔线
func (c *CLI) printSeparator(colWidths []int) {
	fmt.Fprintf(c.term, "+")
	for _, width := range colWidths {
		fmt.Fprintf(c.term, "%s+", strings.Repeat("-", width+2))
	}
	fmt.Fprintf(c.term, "\n")
}

// displayExpanded 以扩展形式显示结果
func (c *CLI) displayExpanded(rows *sql.Rows, cols []string, startTime time.Time) {
	rowNum := 0
	for rows.Next() {
		rowNum++
		vals := make([]interface{}, len(cols))
		valPtrs := make([]interface{}, len(cols))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}
		rows.Scan(valPtrs...)
		
		fmt.Fprintf(c.term, "-[ RECORD %d ]", rowNum)
		fmt.Fprintf(c.term, "%s\n", strings.Repeat("-", 50-len(fmt.Sprintf("-[ RECORD %d ]", rowNum))))
		
		// 找出最长的列名
		maxColLen := 0
		for _, col := range cols {
			if len(col) > maxColLen {
				maxColLen = len(col)
			}
		}
		
		for i, col := range cols {
			var valStr string
			if vals[i] == nil {
				valStr = ""
			} else {
				switch val := vals[i].(type) {
				case []byte:
					valStr = string(val)
				case time.Time:
					valStr = val.Format("2006-01-02 15:04:05")
				case bool:
					if val {
						valStr = "t"
					} else {
						valStr = "f"
					}
				default:
					valStr = fmt.Sprintf("%v", val)
				}
			}
			fmt.Fprintf(c.term, "%-*s | %s\n", maxColLen, col, valStr)
		}
		
		if rowNum >= c.maxRows {
			break
		}
	}
	
	if rowNum == 0 {
		fmt.Fprintf(c.term, "(0 rows)\n")
	}
	
	if c.timingEnabled {
		elapsed := time.Since(startTime).Seconds() * 1000
		fmt.Fprintf(c.term, "Time: %.3f ms\n", elapsed)
	}
	fmt.Fprintf(c.term, "\n")
}

// executeCommand 执行非查询语句
func (c *CLI) executeCommand(ctx context.Context, sqlStr string, startTime time.Time) {
	result, err := c.db.ExecContext(ctx, sqlStr)
	if err != nil {
		c.printError(err)
		return
	}
	
	affected, _ := result.RowsAffected()
	
	// 判断命令类型
	upperSQL := strings.ToUpper(strings.TrimSpace(sqlStr))
	var commandTag string
	switch {
	case strings.HasPrefix(upperSQL, "INSERT"):
		commandTag = "INSERT"
	case strings.HasPrefix(upperSQL, "UPDATE"):
		commandTag = "UPDATE"
	case strings.HasPrefix(upperSQL, "DELETE"):
		commandTag = "DELETE"
	case strings.HasPrefix(upperSQL, "CREATE"):
		commandTag = "CREATE"
	case strings.HasPrefix(upperSQL, "DROP"):
		commandTag = "DROP"
	case strings.HasPrefix(upperSQL, "ALTER"):
		commandTag = "ALTER"
	default:
		commandTag = "COMMAND"
	}
	
	fmt.Fprintf(c.term, "%s %d\n", commandTag, affected)
	
	if c.timingEnabled {
		elapsed := time.Since(startTime).Seconds() * 1000
		fmt.Fprintf(c.term, "Time: %.3f ms\n", elapsed)
	}
	fmt.Fprintf(c.term, "\n")
}

// printError 打印错误信息
func (c *CLI) printError(err error) {
	errMsg := err.Error()
	fmt.Fprintf(c.term, "ERROR: %s\n\n", errMsg)
}

// isQuery 判断是否是查询语句
func isQuery(sqlStr string) bool {
	upper := strings.ToUpper(strings.TrimSpace(sqlStr))
	
	queryPrefixes := []string{
		"SELECT", "SHOW", "WITH", "TABLE", "VALUES",
		"EXPLAIN", "ANALYZE",
	}
	
	for _, prefix := range queryPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	
	return false
}

// ParseInt 安全地解析整数
func parseInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}
