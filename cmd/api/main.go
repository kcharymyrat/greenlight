package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/kcharymyrat/greenlight/internal/data"
	_ "github.com/lib/pq"
)

const version = "1.0.0"

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  string
	}
}

type application struct {
	config config
	logger *log.Logger
	models data.Models
}

func main() {
	var cfg config

	// Create a new logger
	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	// Set appropriate env vars to config struct
	setConfigWithEnvVars(&cfg, logger)

	// Read from terminal and assign config
	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	flag.StringVar(&cfg.db.dsn, "db-dsn", cfg.db.dsn, "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", cfg.db.maxOpenConns, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", cfg.db.maxIdleConns, "PostgreSQL max idle connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", cfg.db.maxIdleTime, "PostgreSQL max connection idle time")
	flag.Parse()

	// Establish db connection
	db, err := openDB(cfg)
	if err != nil {
		logger.Fatal(err)
	}
	defer db.Close()
	logger.Printf("database connection pool established")

	// Dependency Injection
	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModel(db),
	}

	// create router (servemux) for the request, with appropriate handler mapping
	mux := app.routes()

	// create a new server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      mux,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start the server
	logger.Printf("starting %s server on %s", cfg.env, srv.Addr)
	err = srv.ListenAndServe()
	logger.Fatal(err)
}

func setConfigWithEnvVars(cfg *config, logger *log.Logger) {
	// Load envrimental variable
	err := godotenv.Load()
	if err != nil {
		logger.Fatalf("Error loading .env file: %v\n", err)
	}
	dsn := os.Getenv("GREENLIGHT_DB_DSN")
	if dsn == "" {
		logger.Fatalf("Error: GREENLIGHT_DB_DSN environment variable not set in .env file. %v", err)
	}
	maxOpenConns, err := strconv.Atoi(os.Getenv("MAX_OPEN_CONNS"))
	if err != nil {
		logger.Fatalf("Error converting MAX_OPEN_CONNS to integer: %v\n", err)
	}
	maxIdleConns, err := strconv.Atoi(os.Getenv("MAX_IDLE_CONNS"))
	if err != nil {
		logger.Fatalf("Error converting MAX_IDLE_CONNS to integer: %v\n", err)
	}
	maxIdleTime := os.Getenv("MAX_IDLE_TIME")
	if dsn == "" {
		logger.Fatalf("Error: GREENLIGHT_DB_DSN environment variable not set in .env file. %v", err)
	}

	cfg.db.dsn = dsn
	cfg.db.maxOpenConns = maxOpenConns
	cfg.db.maxIdleConns = maxIdleConns
	cfg.db.maxIdleTime = maxIdleTime
}

func openDB(cfg config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	// Set the maximum number of open (in-use + idle) connections in the pool.
	db.SetMaxOpenConns(cfg.db.maxOpenConns)

	// Set the maximum number of open (in-use + idle) connections in the pool.
	db.SetMaxIdleConns(cfg.db.maxIdleConns)

	// Set the maximum timeout for the idle connection. Convert string to time.Duration
	duration, err := time.ParseDuration(cfg.db.maxIdleTime)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxIdleTime(duration)

	// Create a context with a 5-second timeout deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	return db, nil
}
