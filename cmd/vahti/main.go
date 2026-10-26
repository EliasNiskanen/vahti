// Command vahti is a local privacy filter for a home network: a DNS resolver
// that blocks tracking before it leaves the house, with a web interface for
// deciding what each device may share.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
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
	"syscall"
	"time"

	"github.com/EliasNiskanen/vahti/internal/config"
	"github.com/EliasNiskanen/vahti/internal/demo"
	"github.com/EliasNiskanen/vahti/internal/dnsserver"
	"github.com/EliasNiskanen/vahti/internal/engine"
	"github.com/EliasNiskanen/vahti/internal/intel"
	"github.com/EliasNiskanen/vahti/internal/lock"
	"github.com/EliasNiskanen/vahti/internal/sdnotify"
	"github.com/EliasNiskanen/vahti/internal/store"
	"github.com/EliasNiskanen/vahti/internal/web"
)

// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

const defaultConfig = "/etc/vahti/config.json"

func usage() {
	fmt.Fprintf(os.Stderr, `Vahti %s — kotiverkon yksityisyyssuodatin

Käyttö:
  vahti [run] [-config %s]   käynnistä palvelu
  vahti demo [-dir ./vahti-demo]          esittelytila omalle koneelle (http://127.0.0.1:8080)
  vahti update-lists [-config ...]        lataa estolistat (ainoa toiminto, joka hakee dataa verkosta)
  vahti reset-admin [-config ...]         unohda salasana ja luo uusi käyttöönottokoodi
  vahti version
`, version, defaultConfig)
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	cmd := "run"
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "run":
		err = cmdRun(args, false)
	case "demo":
		err = cmdRun(args, true)
	case "update-lists":
		err = cmdUpdateLists(args)
	case "reset-admin":
		err = cmdResetAdmin(args)
	case "version":
		fmt.Println(version)
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func loadConfig(fs *flag.FlagSet, args []string) (config.Config, error) {
	path := fs.String("config", defaultConfig, "asetustiedosto")
	if err := fs.Parse(args); err != nil {
		return config.Config{}, err
	}
	cfg, found, err := config.Load(*path)
	if err != nil {
		return cfg, err
	}
	if !found {
		slog.Info("config file not found; using defaults", "path", *path)
	}
	return cfg, nil
}

func buildDB(cfg config.Config, demoMode bool) *intel.DB {
	db := intel.New()
	for _, l := range cfg.Lists {
		info := db.LoadListFile(filepath.Join(cfg.DataDir, "lists", l.Name+".txt"), l.Name, l.Category)
		if info.Error != "" {
			slog.Warn("blocklist not loaded", "list", l.Name, "reason", info.Error)
		} else {
			slog.Info("blocklist loaded", "list", l.Name, "entries", info.Entries)
		}
	}
	if demoMode {
		demo.SeedIntel(db)
	}
	return db
}

func cmdRun(args []string, demoMode bool) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	var cfg config.Config
	var err error
	if demoMode {
		dir := fs.String("dir", "./vahti-demo", "esittelytilan datakansio")
		webAddr := fs.String("web", "127.0.0.1:8080", "käyttöliittymän osoite")
		dnsAddr := fs.String("dns", "127.0.0.1:5353", "nimipalvelimen osoite (esim. dig @127.0.0.1 -p 5353)")
		if err := fs.Parse(args); err != nil {
			return err
		}
		cfg = config.Default()
		cfg.DataDir = *dir
		cfg.DNS.Listen = []string{*dnsAddr}
		cfg.Web.Listen = *webAddr
		cfg.Web.RedirectListen = ""
		cfg.Web.TLS = false
		cfg.DNS.Upstreams = []string{"udp://192.0.2.1:53"} // never used: demo answers locally
	} else {
		cfg, err = loadConfig(fs, args)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("data directory: %w", err)
	}
	lk, err := lock.Acquire(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("%w (%s)", err, cfg.DataDir)
	}
	defer lk.Close()

	statePath := filepath.Join(cfg.DataDir, "state.json")
	st := engine.NewState()
	usedBackup, err := store.LoadJSON(statePath, st)
	switch {
	case errors.Is(err, store.ErrNotExist):
		slog.Info("first start: creating new state", "path", statePath)
	case err != nil:
		return fmt.Errorf("state file unreadable (restore %s.bak or remove it to start fresh): %w", statePath, err)
	case usedBackup:
		slog.Warn("state file was damaged; restored the previous copy", "path", statePath)
	}

	db := buildDB(cfg, demoMode)
	eng := engine.New(st, db, cfg.LogEntries)

	setupPath := filepath.Join(cfg.DataDir, "setup-code")
	if eng.Admin().PasswordHash == "" {
		code := ""
		if b, err := os.ReadFile(setupPath); err == nil && eng.Admin().SetupCodeHash == web.HashSetupCode(strings.TrimSpace(string(b))) {
			code = strings.TrimSpace(string(b))
		} else {
			var hash string
			code, hash = web.NewSetupCode()
			if err := store.WriteFileAtomic(setupPath, []byte(code+"\n"), 0o600); err != nil {
				return err
			}
			eng.SetAdmin(engine.Admin{SetupCodeHash: hash})
		}
		slog.Info("VAHTI IS NOT SET UP YET. Open the web interface and enter this one-time setup code", "code", code, "file", setupPath)
		if demoMode {
			fmt.Fprintf(os.Stderr, "\n  Avaa http://%s ja syötä käyttöönottokoodi: %s\n\n", cfg.Web.Listen, code)
		}
	}
	if err := saveState(eng, statePath, true); err != nil {
		return err
	}

	var cert tls.Certificate
	var fp string
	if cfg.Web.TLS {
		cert, fp, err = web.EnsureCert(cfg.DataDir, cfg.Web.Hostnames)
		if err != nil {
			return fmt.Errorf("tls certificate: %w", err)
		}
		slog.Info("web certificate", "sha256", fp)
	}

	opts := dnsserver.Options{}
	if demoMode {
		opts.Upstream = dnsserver.ExchangeFunc(demo.Upstream())
		opts.MACOf = demo.MACOf
	}
	dnsSrv, err := dnsserver.New(cfg.DNS, cfg.Web.Hostnames, eng, opts)
	if err != nil {
		return err
	}
	if err := dnsSrv.Start(cfg.DNS.Listen); err != nil {
		return err
	}
	defer dnsSrv.Shutdown()
	slog.Info("dns listening", "addrs", dnsSrv.Addrs())

	webSrv, err := web.New(web.Config{
		AllowedClients: cfg.Web.AllowedClients, Hostnames: cfg.Web.Hostnames, TLS: cfg.Web.TLS, CertFP: fp,
		SetupCodePath: setupPath, Demo: demoMode, Version: version, Upstreams: cfg.DNS.Upstreams,
	}, eng)
	if err != nil {
		return err
	}
	httpSrv := &http.Server{
		Addr: cfg.Web.Listen, Handler: webSrv.Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second,
		IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
		ErrorLog: slog.NewLogLogger(slog.Default().Handler(), slog.LevelDebug),
	}
	if cfg.Web.TLS {
		httpSrv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	}
	ln, err := net.Listen("tcp", cfg.Web.Listen)
	if err != nil {
		return fmt.Errorf("web %s: %w", cfg.Web.Listen, err)
	}
	errc := make(chan error, 2)
	go func() {
		if cfg.Web.TLS {
			errc <- httpSrv.ServeTLS(ln, "", "")
		} else {
			errc <- httpSrv.Serve(ln)
		}
	}()
	scheme := "http"
	if cfg.Web.TLS {
		scheme = "https"
	}
	if demoMode {
		slog.Info("web interface listening", "url", "http://"+cfg.Web.Listen)
	} else {
		slog.Info("web interface listening", "url", scheme+"://"+cfg.Web.Hostnames[0], "addr", cfg.Web.Listen)
	}

	var redirect *http.Server
	if cfg.Web.TLS && cfg.Web.RedirectListen != "" {
		redirect = &http.Server{Addr: cfg.Web.RedirectListen, Handler: webSrv.RedirectHandler(),
			ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, MaxHeaderBytes: 8 << 10}
		go func() {
			if err := redirect.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Warn("http redirect not available", "err", err)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Persist changes every few seconds; a failed save is retried.
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := saveState(eng, statePath, false); err != nil {
					slog.Error("saving state failed; will retry", "err", err)
				}
			}
		}
	}()

	// Reload blocklists on SIGHUP (sent by "systemctl reload vahti").
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for range hup {
			slog.Info("reloading blocklists")
			db.Replace(buildDB(cfg, demoMode))
		}
	}()

	// systemd watchdog: only report healthy when DNS actually answers.
	if iv := sdnotify.WatchdogInterval(); iv > 0 {
		addr := selfCheckAddr(cfg.DNS.Listen[0])
		go func() {
			t := time.NewTicker(iv)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					if err := dnsserver.SelfCheck(addr); err != nil {
						slog.Error("dns self-check failed", "err", err)
						continue
					}
					_ = sdnotify.Notify("WATCHDOG=1")
				}
			}
		}()
	}

	if demoMode {
		demo.Prepare(eng)
		go demo.Run(ctx, dnsSrv, 160)
	}
	_ = sdnotify.Notify("READY=1")

	select {
	case <-ctx.Done():
	case err := <-errc:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("web server stopped", "err", err)
		}
	}
	_ = sdnotify.Notify("STOPPING=1")
	slog.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(sctx)
	if redirect != nil {
		_ = redirect.Shutdown(sctx)
	}
	dnsSrv.Shutdown()
	return saveState(eng, statePath, true)
}

func saveState(eng *engine.Engine, path string, force bool) error {
	s, changed := eng.Snapshot(force)
	if !changed {
		return nil
	}
	if err := store.SaveJSON(path, s); err != nil {
		eng.MarkDirty()
		return err
	}
	return nil
}

func selfCheckAddr(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "127.0.0.1:53"
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

// ---------------------------------------------------------------- update-lists

func cmdUpdateLists(args []string) error {
	fs := flag.NewFlagSet("update-lists", flag.ExitOnError)
	cfg, err := loadConfig(fs, args)
	if err != nil {
		return err
	}
	dir := filepath.Join(cfg.DataDir, "lists")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	lock.MatchOwner(cfg.DataDir, dir)
	client := &http.Client{Timeout: 2 * time.Minute}
	failed := 0
	for _, l := range cfg.Lists {
		n, err := downloadList(client, l, filepath.Join(dir, l.Name+".txt"))
		if err != nil {
			failed++
			slog.Error("list update failed", "list", l.Name, "err", err)
			continue
		}
		lock.MatchOwner(cfg.DataDir, filepath.Join(dir, l.Name+".txt"))
		slog.Info("list updated", "list", l.Name, "domains", n)
	}
	fmt.Println("Valmis. Ota listat käyttöön: sudo systemctl reload vahti")
	if failed > 0 {
		return fmt.Errorf("%d list(s) failed", failed)
	}
	return nil
}

func downloadList(c *http.Client, l config.List, dest string) (int, error) {
	req, err := http.NewRequest(http.MethodGet, l.URL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Vahti/"+version)
	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("server answered %s", resp.Status)
	}
	const max = 64 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return 0, err
	}
	if len(body) > max {
		return 0, errors.New("list is larger than 64 MB")
	}
	domains, err := intel.ParseList(bytes.NewReader(body), 2_000_000)
	if err != nil {
		return 0, err
	}
	if len(domains) == 0 {
		return 0, errors.New("list contained no domains")
	}
	out := []byte(fmt.Sprintf("# %s\n# source: %s\n# fetched: %s\n%s\n", l.Name, l.URL, time.Now().UTC().Format(time.RFC3339), strings.Join(domains, "\n")))
	return len(domains), store.WriteFileAtomic(dest, out, 0o644)
}

// ---------------------------------------------------------------- reset-admin

func cmdResetAdmin(args []string) error {
	fs := flag.NewFlagSet("reset-admin", flag.ExitOnError)
	cfg, err := loadConfig(fs, args)
	if err != nil {
		return err
	}
	lk, err := lock.Acquire(cfg.DataDir)
	if errors.Is(err, lock.ErrBusy) {
		return errors.New("Vahti on käynnissä. Pysäytä se ensin: sudo systemctl stop vahti")
	}
	if err != nil {
		return err
	}
	defer lk.Close()
	path := filepath.Join(cfg.DataDir, "state.json")
	st := engine.NewState()
	if _, err := store.LoadJSON(path, st); err != nil && !errors.Is(err, store.ErrNotExist) {
		return err
	}
	code, hash := web.NewSetupCode()
	st.Admin = engine.Admin{SetupCodeHash: hash}
	if err := store.SaveJSON(path, st); err != nil {
		return err
	}
	if err := store.WriteFileAtomic(filepath.Join(cfg.DataDir, "setup-code"), []byte(code+"\n"), 0o600); err != nil {
		return err
	}
	lock.MatchOwner(cfg.DataDir, path, path+".bak", filepath.Join(cfg.DataDir, "setup-code"), filepath.Join(cfg.DataDir, ".lock"))
	fmt.Printf("Salasana poistettu. Uusi käyttöönottokoodi: %s\nKäynnistä Vahti: sudo systemctl start vahti\n", code)
	return nil
}
