package main

import (
	"fmt"
	"log"
	"os"

	postgrescli "binrc.com/dbcli/postgres-cli"
)

type Terminal struct {
	*os.File
}

func (t *Terminal) Read(p []byte) (n int, error) {
	return os.Stdin.Read(p)
}

func (t *Terminal) Write(p []byte) (n int, error) {
	return os.Stdout.Write(p)
}

func main() {
	term := &Terminal{os.Stdout}

	cli := postgrescli.NewCLI(
		term,
		"localhost", // host
		5432,        // port
		"postgres",  // username
		"password",  // password
		"testdb",    // database
	)

	if err := cli.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer cli.Close()

	fmt.Println("Connected to PostgreSQL!")

	if err := cli.Start(); err != nil {
		log.Fatalf("CLI error: %v", err)
	}
}

