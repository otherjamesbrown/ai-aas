package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

func main() {
	var dsn, file string
	flag.StringVar(&dsn, "dsn", os.Getenv("DB_URL"), "PostgreSQL connection string")
	flag.StringVar(&file, "file", "", "Path to SQL file")
	flag.Parse()

	if dsn == "" {
		log.Fatal("DSN not provided")
	}
	if file == "" {
		log.Fatal("SQL file not provided")
	}

	sqlBytes, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatalf("read sql file: %v", err)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	statements := splitStatements(string(sqlBytes))
	for _, stmt := range statements {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			log.Fatalf("exec failed: %v", err)
		}
	}
}

func splitStatements(sqlText string) []string {
	sqlText = strings.ReplaceAll(sqlText, "\r", "")
	var statements []string
	for _, stmt := range strings.Split(sqlText, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		statements = append(statements, stmt)
	}
	return statements
}
