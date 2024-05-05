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
	"github.com/kcharymyrat/greenlight/internal/jsonlog"
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
	logger *jsonlog.Logger
	models data.Models
}

func main() {
	var cfg config

	// Create a new logger
	logger := jsonlog.NewLogger(os.Stdout, jsonlog.LevelInfo)

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
		logger.PrintFatal(err, nil)
	}
	defer db.Close()
	logger.PrintInfo("database connection pool established", nil)

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
		ErrorLog:     log.New(logger, "", 0),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start the server
	properties := map[string]string{
		"addr": srv.Addr,
		"env":  cfg.env,
	}
	logger.PrintInfo("starting server", properties)
	err = srv.ListenAndServe()
	logger.PrintFatal(err, nil)
}

func setConfigWithEnvVars(cfg *config, logger *jsonlog.Logger) {
	// Load envrimental variable
	err := godotenv.Load()
	if err != nil {
		message := fmt.Sprintf("Error loading (environmental variables) .env file: %v\n", err)
		logger.PrintFatal(err, map[string]string{"msg": message})
	}

	dsn := os.Getenv("GREENLIGHT_DB_DSN")
	if dsn == "" {
		message := fmt.Sprintf("Error: GREENLIGHT_DB_DSN environment variable not set in .env file. %v", err)
		logger.PrintFatal(err, map[string]string{"msg": message})
	}

	maxOpenConns, err := strconv.Atoi(os.Getenv("MAX_OPEN_CONNS"))
	if err != nil {
		message := fmt.Sprintf("Error converting MAX_OPEN_CONNS to integer: %v\n", err)
		logger.PrintFatal(err, map[string]string{"msg": message})
	}

	maxIdleConns, err := strconv.Atoi(os.Getenv("MAX_IDLE_CONNS"))
	if err != nil {
		message := fmt.Sprintf("Error converting MAX_IDLE_CONNS to integer: %v\n", err)
		logger.PrintFatal(err, map[string]string{"msg": message})
	}

	maxIdleTime := os.Getenv("MAX_IDLE_TIME")
	if dsn == "" {
		message := fmt.Sprintf("Error: GREENLIGHT_DB_DSN environment variable not set in .env file. %v", err)
		logger.PrintFatal(err, map[string]string{"msg": message})
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
