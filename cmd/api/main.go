package main

import (
	"context"
	"database/sql"
	"expvar"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/kcharymyrat/greenlight/internal/data"
	"github.com/kcharymyrat/greenlight/internal/jsonlog"
	"github.com/kcharymyrat/greenlight/internal/mailer"
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
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	cors struct {
		trustedOrigins []string
	}
}

type application struct {
	config config
	logger *jsonlog.Logger
	models data.Models
	mailer mailer.Mailer
	wg     sync.WaitGroup
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

	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	flag.StringVar(&cfg.smtp.host, "smtp-host", cfg.smtp.host, "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", cfg.smtp.port, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", cfg.smtp.username, "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", cfg.smtp.password, "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", cfg.smtp.sender, "SMTP sender")
	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		trustedOriginsInput := strings.Fields(val)
		if len(trustedOriginsInput) > 0 {
			cfg.cors.trustedOrigins = trustedOriginsInput
		}
		return nil
	})
	flag.Parse()

	// Establish db connection
	db, err := openDB(cfg)
	if err != nil {
		logger.PrintFatal(err, nil)
	}
	defer db.Close()
	logger.PrintInfo("database connection pool established", nil)

	// Publish version in metrics
	expvar.NewString("version").Set(version)

	// Publish the number of active goroutines
	expvar.Publish("gorroutines", expvar.Func(func() interface{} {
		return runtime.NumGoroutine()
	}))

	// Publish the database connection pool statistics
	expvar.Publish("database", expvar.Func(func() interface{} {
		return db.Stats()
	}))

	// Publish the current Unix timestamp
	expvar.Publish("timestamp", expvar.Func(func() interface{} {
		return time.Now().Unix()
	}))

	// Dependency Injection
	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModel(db),
		mailer: mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
	}

	err = app.serve()
	if err != nil {
		logger.PrintFatal(err, nil)
	}
}

func setConfigWithEnvVars(cfg *config, logger *jsonlog.Logger) {
	// Load envrimental variable
	err := godotenv.Load()
	if err != nil {
		message := fmt.Sprintf("Error loading (environmental variables): %v\n", err)
		logger.PrintFatal(err, map[string]string{"msg": message})
	}

	dsn := os.Getenv("GREENLIGHT_DB_DSN")
	if dsn == "" {
		message := fmt.Sprintf("Error: GREENLIGHT_DB_DSN environment variable not set. %v", err)
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
	if maxIdleTime == "" {
		message := fmt.Sprintf("Error: MAX_IDLE_TIME environment variable not set. %v", err)
		logger.PrintFatal(err, map[string]string{"msg": message})
	}

	mailTrapHost := os.Getenv("MAILTRAP_HOST")
	if mailTrapHost == "" {
		message := fmt.Sprintf("Error: MAILTRAP_HOST environment variable not set. %v", err)
		logger.PrintFatal(err, map[string]string{"msg": message})
	}

	mailTrapPort, err := strconv.Atoi(os.Getenv("MAILTRAP_PORT"))
	if err != nil {
		message := fmt.Sprintf("Error converting MAILTRAP_PORT to integer: %v\n", err)
		logger.PrintFatal(err, map[string]string{"msg": message})
	}

	mailTrapUsername := os.Getenv("MAILTRAP_USERNAME")
	if mailTrapUsername == "" {
		message := fmt.Sprintf("Error: MAILTRAP_USERNAME environment variable not set. %v", err)
		logger.PrintFatal(err, map[string]string{"msg": message})
	}

	mailTrapPassword := os.Getenv("MAILTRAP_PASSWORD")
	if mailTrapPassword == "" {
		message := fmt.Sprintf("Error: MAILTRAP_PASSWORD environment variable not set. %v", err)
		logger.PrintFatal(err, map[string]string{"msg": message})
	}

	mailTrapSender := os.Getenv("MAILTRAP_SENDER")
	if mailTrapSender == "" {
		message := fmt.Sprintf("Error: MAILTRAP_SENDER environment variable not set. %v", err)
		logger.PrintFatal(err, map[string]string{"msg": message})
	}

	corsTrustedOrigins := os.Getenv("CORS_TRUSTED_ORIGINS")

	cfg.db.dsn = dsn
	cfg.db.maxOpenConns = maxOpenConns
	cfg.db.maxIdleConns = maxIdleConns
	cfg.db.maxIdleTime = maxIdleTime
	cfg.smtp.host = mailTrapHost
	cfg.smtp.port = mailTrapPort
	cfg.smtp.username = mailTrapUsername
	cfg.smtp.password = mailTrapPassword
	cfg.smtp.sender = mailTrapSender
	cfg.cors.trustedOrigins = strings.Fields(corsTrustedOrigins)
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
