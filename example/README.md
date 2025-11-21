# PostgreSQL CLI 使用示例

本目录包含 PostgreSQL CLI 的使用示例。

## 文件说明

- `basic_usage.go` - 基本使用示例
- `custom_config.go` - 自定义配置示例

## 运行示例

```bash
# 基本使用
go run basic_usage.go

# 自定义配置
go run custom_config.go
```

## 配置参数

### 基本参数
- `Host` - 数据库主机
- `Port` - 端口（默认 5432）
- `Username` - 用户名
- `Password` - 密码
- `Database` - 数据库名

### 高级参数
- `SSLMode` - SSL模式（disable/require/verify-ca/verify-full）
- `ConnectTimeout` - 连接超时
- `StatementTimeout` - 语句超时
- `MaxOpenConns` - 最大连接数
- `MaxIdleConns` - 最大空闲连接
- `ConnMaxLifetime` - 连接最大生命周期
- `ApplicationName` - 应用名称
- `SearchPath` - 搜索路径
- `TimeZone` - 时区
- `CustomParams` - 自定义参数

## 支持的命令

### psql 特殊命令
- `\?` - 显示帮助
- `\q` - 退出
- `\l` - 列出数据库
- `\c <db>` - 连接数据库
- `\dt` - 列出表
- `\d <table>` - 描述表
- `\dv` - 列出视图
- `\di` - 列出索引
- `\du` - 列出用户
- `\x` - 切换扩展显示
- `\timing` - 切换计时

### SQL 命令
- `SELECT` - 查询
- `INSERT` - 插入
- `UPDATE` - 更新
- `DELETE` - 删除
- `CREATE` - 创建
- `DROP` - 删除
- `ALTER` - 修改

### 事务命令
- `BEGIN` - 开始事务
- `COMMIT` - 提交
- `ROLLBACK` - 回滚


