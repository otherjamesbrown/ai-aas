package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

type paramMap map[string]string

func (p *paramMap) String() string {
	if p == nil {
		return ""
	}
	var parts []string
	for k, v := range *p {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ",")
}

func (p *paramMap) Set(value string) error {
	if p == nil {
		return fmt.Errorf("param map not initialised")
	}
	if value == "" {
		return fmt.Errorf("param must be key=value")
	}
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("param must be key=value")
	}
	key := strings.TrimSpace(parts[0])
	if key == "" {
		return fmt.Errorf("param key empty")
	}
	(*p)[strings.ToUpper(key)] = parts[1]
	return nil
}

func main() {
	var dsn, file string
	params := paramMap{}
	flag.StringVar(&dsn, "dsn", os.Getenv("DB_URL"), "PostgreSQL connection string")
	flag.StringVar(&file, "file", "", "Path to SQL file")
	flag.Var(&params, "param", "Template parameter in the form key=value; replaces {{KEY}} tokens")
	flag.Parse()

	if dsn == "" {
		log.Fatal("DSN not provided")
	}
	if file == "" {
		log.Fatal("SQL file not provided")
	}

	sqlBytes, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("read sql file: %v", err)
	}

	sqlText := string(sqlBytes)
	for key, value := range params {
		token := "{{" + key + "}}"
		sqlText = strings.ReplaceAll(sqlText, token, value)
		sqlText = strings.ReplaceAll(sqlText, "{{ "+key+" }}", value)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	statements := splitStatements(sqlText)
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
