# PostgreSQL CLI

[![Go Reference](https://pkg.go.dev/badge/github.com/binrchq/postgres-cli.svg)](https://pkg.go.dev/github.com/binrchq/postgres-cli)
[![Go Report Card](https://goreportcard.com/badge/github.com/binrchq/postgres-cli)](https://goreportcard.com/report/github.com/binrchq/postgres-cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A standalone, feature-rich PostgreSQL interactive CLI client for Go applications.

## Features

- 🚀 Full PostgreSQL protocol support
- 🔧 Customizable connection parameters
- 🔒 SSL/TLS support (disable/require/verify-ca/verify-full)
- ⏱️ Query timing with `\timing`
- 📊 Expanded display mode with `\x`
- 🔄 Transaction support
- 📝 Multi-line SQL input
- 💾 Connection pooling
- 🌍 Timezone and search path configuration

## Installation

```bash
go get github.com/binrchq/postgres-cli
```

## Quick Start

```go
package main

import (
    "log"
    "os"
    
    postgrescli "github.com/binrchq/postgres-cli"
)

func main() {
    cli := postgrescli.NewCLI(
        os.Stdin,
        "localhost",
        5432,
        "postgres",
        "password",
        "mydb",
    )
    
    if err := cli.Connect(); err != nil {
        log.Fatal(err)
    }
    defer cli.Close()
    
    if err := cli.Start(); err != nil {
        log.Fatal(err)
    }
}
```

## Advanced Configuration

```go
config := &postgrescli.Config{
    Host:             "localhost",
    Port:             5432,
    Username:         "postgres",
    Password:         "password",
    Database:         "mydb",
    SSLMode:          "require",
    ConnectTimeout:   15 * time.Second,
    StatementTimeout: 60 * time.Second,
    MaxOpenConns:     20,
    ApplicationName:  "my-app",
    SearchPath:       "public,myschema",
    TimeZone:         "Asia/Shanghai",
}

cli := postgrescli.NewCLIWithConfig(terminal, config)
```

## psql Commands

- `\?` - Show help
- `\q` - Quit
- `\l` - List databases
- `\c <db>` - Connect to database
- `\dt` - List tables
- `\d <table>` - Describe table
- `\dv` - List views
- `\di` - List indexes
- `\du` - List users
- `\x` - Toggle expanded display
- `\timing` - Toggle timing

## Requirements

- Go 1.21 or higher
- PostgreSQL 9.6 or higher

## Dependencies

- [github.com/lib/pq](https://github.com/lib/pq) - PostgreSQL driver
- [github.com/chzyer/readline](https://github.com/chzyer/readline) - Readline library

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Author

Maintained by [binrc](https://github.com/binrchq).

## Related Projects

- [mysql-cli](https://github.com/binrchq/mysql-cli) - MySQL CLI
- [redis-cli](https://github.com/binrchq/redis-cli) - Redis CLI
- [mongodb-cli](https://github.com/binrchq/mongodb-cli) - MongoDB CLI
