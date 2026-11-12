package main

import (
	"bufio"
	"business-manage/api"
	"business-manage/internal/adapters/printing"
	"business-manage/internal/adapters/sqlite"
	"business-manage/internal/application"
	"business-manage/internal/core"
	"business-manage/internal/operations"
	"business-manage/internal/platform"
	"business-manage/internal/transport/httpapi"
	"business-manage/internal/webui"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"
)

var version = "0.4.0-dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ERP:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	command := "serve"
	if len(args) > 0 {
		command = args[0]
		args = args[1:]
	}
	if command == "version" {
		fmt.Println(version)
		return nil
	}
	if command == "help" || command == "--help" || command == "-h" {
		fmt.Println("Raíz ERP · init | serve | backup | restore | doctor | certificates | reset-password | service | version\nUsa 'erp <comando> --help' para ver sus opciones.")
		return nil
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	dataDir := flags.String("data-dir", "./data", "Directorio de datos persistentes")
	var listen, hosts, archive, username *string
	var useTLS, allowWeakPassword *bool
	if command == "init" {
		listen = flags.String("listen", "127.0.0.1:8080", "Dirección IP:puerto")
		hosts = flags.String("hosts", "", "Nombres o IP del servidor separados por coma")
		useTLS = flags.Bool("tls", false, "Activar HTTPS también en loopback")
	}
	if command == "certificates" {
		hosts = flags.String("hosts", "", "Nombres o IP del servidor separados por coma")
	}
	if command == "restore" {
		archive = flags.String("file", "", "Archivo ZIP que se restaurará")
	}
	if command == "reset-password" {
		allowWeakPassword = flags.Bool("allow-weak-password", false, "Aceptar explícitamente una contraseña corta o vacía")
		username = flags.String("username", "", "Usuario cuya contraseña se reemplazará; nueva contraseña por stdin")
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("argumentos inesperados: %v", flags.Args())
	}
	root, err := filepath.Abs(*dataDir)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch command {
	case "certificates":
		lock, e := platform.Acquire(root)
		if e != nil {
			return e
		}
		defer lock.Close()
		c, e := operations.ReadConfig(root)
		if e != nil {
			return e
		}
		store, e := sqlite.Open(filepath.Join(root, "erp.db"))
		if e != nil {
			return e
		}
		if _, companyErr := store.Read().Admin.Company(ctx); companyErr == nil {
			_, e = operations.Backup(ctx, store, root, c)
		}
		store.Close()
		if e != nil {
			return e
		}
		if e = operations.GenerateCertificates(root, strings.Split(*hosts, ",")); e != nil {
			return e
		}
		fmt.Println("Certificado del servidor renovado; la autoridad local se conserva.")
		return nil
	case "serve", "service":
		fn := func(ctx context.Context) error { return serve(ctx, root) }
		if command == "service" {
			return platform.RunService(ctx, fn)
		}
		return fn(ctx)
	case "init":
		lock, err := platform.Acquire(root)
		if err != nil {
			return err
		}
		defer lock.Close()
		if err = initialize(root, *listen, *useTLS, strings.Split(*hosts, ",")); err != nil {
			return err
		}
		fmt.Println("Instalación preparada. Ejecuta: erp serve --data-dir", root)
		return nil
	case "restore":
		if *archive == "" {
			return fmt.Errorf("indica --file <respaldo.zip>")
		}
		lock, err := platform.Acquire(root)
		if err != nil {
			return err
		}
		defer lock.Close()
		// Preserve the chosen archive before the safety backup rotates retention.
		// In particular, restoring the oldest retained copy must remain possible.
		input, err := os.Open(*archive)
		if err != nil {
			return err
		}
		defer input.Close()
		staged, err := os.CreateTemp(filepath.Dir(root), ".erp-restore-input-*.zip")
		if err != nil {
			return err
		}
		defer staged.Close()
		defer os.Remove(staged.Name())
		if _, err = io.Copy(staged, input); err != nil {
			return err
		}
		if err = staged.Sync(); err != nil {
			return err
		}
		if err = staged.Close(); err != nil {
			return err
		}
		input.Close()
		if c, e := operations.ReadConfig(root); e == nil {
			store, e := sqlite.Open(filepath.Join(root, "erp.db"))
			if e != nil {
				return e
			}
			_, e = operations.Backup(ctx, store, root, c)
			store.Close()
			if e != nil {
				return fmt.Errorf("respaldo de seguridad previo a restaurar: %w", e)
			}
		}
		previous, err := operations.Restore(ctx, staged.Name(), root)
		if err != nil {
			return err
		}
		fmt.Println("Restauración validada. Las sesiones anteriores se han cerrado.")
		if previous != "" {
			fmt.Println("Directorio anterior conservado:", previous)
		}
		return nil
	case "backup", "doctor", "reset-password":
		var lock *platform.Lock
		if command == "reset-password" {
			lock, err = platform.Acquire(root)
			if err != nil {
				return err
			}
			defer lock.Close()
		}
		c, err := operations.ReadConfig(root)
		if err != nil {
			return err
		}
		if _, err = os.Stat(filepath.Join(root, "erp.db")); err != nil {
			return err
		}
		store, err := sqlite.Open(filepath.Join(root, "erp.db"))
		if err != nil {
			return err
		}
		defer store.Close()
		v, err := store.Version(ctx)
		if err != nil {
			return err
		}
		if v != sqlite.SchemaVersion {
			return fmt.Errorf("versión de esquema incompatible: %d", v)
		}
		if command == "backup" {
			path, err := operations.Backup(ctx, store, root, c)
			if err != nil {
				return err
			}
			fmt.Println(path)
			return nil
		}
		if command == "reset-password" {
			if *username == "" {
				return fmt.Errorf("indica --username; la contraseña se lee desde stdin")
			}
			if *allowWeakPassword {
				fmt.Fprintln(os.Stderr, "Advertencia: una contraseña corta puede ser fácil de adivinar. Si queda vacía, cualquier persona con acceso al sistema que conozca este usuario podrá entrar sin contraseña.")
			}
			line, err := bufio.NewReader(io.LimitReader(os.Stdin, 1024)).ReadString('\n')
			if err != nil && err != io.EOF {
				return err
			}
			if err = application.New(store).ResetPassword(ctx, *username, strings.TrimRight(line, "\r\n"), *allowWeakPassword); err != nil {
				return err
			}
			fmt.Println("Contraseña actualizada; sesiones revocadas.")
			return nil
		}
		if err = store.Integrity(ctx); err != nil {
			return err
		}
		var journal, sqliteVersion string
		var foreignKeys, synchronous int
		store.DB.QueryRow("PRAGMA journal_mode").Scan(&journal)
		store.DB.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys)
		store.DB.QueryRow("PRAGMA synchronous").Scan(&synchronous)
		store.DB.QueryRow("SELECT sqlite_version()").Scan(&sqliteVersion)
		_, companyErr := store.Read().Admin.Company(ctx)
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "ok", "version": version, "schemaVersion": v, "sqliteVersion": sqliteVersion, "journalMode": journal, "foreignKeys": foreignKeys == 1, "synchronous": synchronous, "initialized": companyErr == nil, "backupDue": operations.BackupDue(c, root)})
	default:
		return fmt.Errorf("comando desconocido: %s; ejecuta 'erp help'", command)
	}
}
func initialize(root, listen string, useTLS bool, hosts []string) error {
	if _, err := os.Stat(filepath.Join(root, "config.json")); err == nil {
		return fmt.Errorf("la instalación ya existe; edita config.json con el servicio detenido")
	} else if !os.IsNotExist(err) {
		return err
	}
	c := operations.Config{Listen: listen, TLS: useTLS || !operations.LoopbackListen(listen), BackupDirectory: "../" + filepath.Base(root) + "-backups", BackupRetention: 7}
	if err := c.Validate(); err != nil {
		return err
	}
	if _, err := c.BackupPath(root); err != nil {
		return err
	}
	if err := operations.GenerateCertificates(root, hosts); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "setup.token"), []byte(core.Token()), 0600); err != nil {
		return err
	}
	return operations.WriteConfig(root, c)
}
func serve(ctx context.Context, root string) error {
	lock, err := platform.Acquire(root)
	if err != nil {
		return err
	}
	defer lock.Close()
	if _, err = os.Stat(filepath.Join(root, "config.json")); os.IsNotExist(err) {
		if err = initialize(root, "127.0.0.1:8080", false, nil); err != nil {
			return err
		}
	}
	c, err := operations.ReadConfig(root)
	if err != nil {
		return err
	}
	if _, err = c.BackupPath(root); err != nil {
		return err
	}
	logFile, err := operations.NewLogWriter(filepath.Join(root, "logs"))
	if err != nil {
		return err
	}
	defer logFile.Close()
	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(logFile, os.Stdout), nil))
	store, err := sqlite.Open(filepath.Join(root, "erp.db"))
	if err != nil {
		return err
	}
	defer store.Close()
	if err = store.Migrate(ctx, func() error { _, e := operations.Backup(ctx, store, root, c); return e }); err != nil {
		return err
	}
	app := application.New(store)
	app.Log = logger
	app.Printer = printing.New()
	if err = app.RecoverPrintJobs(ctx); err != nil {
		return err
	}
	var backupMu sync.Mutex
	backup := func(ctx context.Context) (string, error) {
		backupMu.Lock()
		defer backupMu.Unlock()
		if _, e := store.Read().Admin.Company(ctx); e != nil {
			return "", core.Conflict("Completa la configuración de la empresa antes de respaldar.")
		}
		path, e := operations.Backup(ctx, store, root, c)
		if e == nil {
			logger.Info("backup_complete", "filename", filepath.Base(path))
		} else {
			logger.Error("backup_failed", "error", e)
		}
		return path, e
	}
	handler := httpapi.New(app, httpapi.Options{DataDir: root, Log: logger, UI: webui.Handler(), OpenAPI: api.OpenAPI(), Ping: store.DB.PingContext, Backup: backup, Version: version})
	server := &http.Server{Addr: c.Listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 2 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	listener, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	scheme := "http"
	if c.TLS {
		cert, err := tls.LoadX509KeyPair(filepath.Join(root, "certificates/server.crt"), filepath.Join(root, "certificates/server.key"))
		if err != nil {
			return err
		}
		listener = tls.NewListener(listener, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
		scheme = "https"
	}
	if _, e := store.Read().Admin.Company(ctx); core.HasCode(e, "not_found") {
		token, e := os.ReadFile(filepath.Join(root, "setup.token"))
		if e != nil {
			return fmt.Errorf("falta setup.token para una instalación sin configurar: %w", e)
		}
		_, port, _ := net.SplitHostPort(c.Listen)
		fmt.Printf("\nCompleta el alta inicial desde este servidor:\n%s://localhost:%s/#setup-token=%s\n\n", scheme, port, strings.TrimSpace(string(token)))
	}
	logger.Info("server_started", "listen", c.Listen, "tls", c.TLS, "version", version)
	jobsCtx, cancelJobs := context.WithCancel(ctx)
	printerDone := make(chan struct{})
	go func() { defer close(printerDone); app.RunPrinter(jobsCtx, logger) }()
	defer func() {
		cancelJobs()
		select {
		case <-printerDone:
		case <-time.After(5 * time.Second):
			logger.Warn("printer_shutdown_pending")
		}
	}()
	var jobs sync.WaitGroup
	jobs.Add(1)
	go func() {
		defer jobs.Done()
		timer := time.NewTimer(time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-jobsCtx.Done():
				return
			case <-timer.C:
				if operations.BackupDue(c, root) {
					_, _ = backup(jobsCtx)
				}
				timer.Reset(time.Minute)
			}
		}
	}()
	defer func() { cancelJobs(); jobs.Wait() }()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err = <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		cancelJobs()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err = server.Shutdown(shutdownCtx); err != nil {
			server.Close()
			return err
		}
		logger.Info("server_stopped")
		return nil
	}
}
