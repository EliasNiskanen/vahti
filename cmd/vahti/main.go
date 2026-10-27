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
	"net/netip"
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

// usage prints the list of commands and how to use them.
func usage() {
	fmt.Fprintf(os.Stderr, `Vahti %s - privacy filter for your home network

Usage:
  vahti [run] [-config %s]   run the service
  vahti demo [-dir ./vahti-demo]          demo mode on your own computer (http://127.0.0.1:8080)
  vahti local [-router 192.168.1.1]       experimental: filter only this computer's traffic
  vahti update-lists [-config ...|-local] download blocklists (the only command that fetches data from the internet)
  vahti reset-admin [-config ...]         forget the password and create a new setup code
  vahti version
`, version, defaultConfig)
}

// main picks the command (run, demo, local, update-lists, reset-admin, version) from
// the first argument and runs it. Without a command Vahti runs as the service.
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
		err = cmdRun(args, modeService)
	case "demo":
		err = cmdRun(args, modeDemo)
	case "local":
		err = cmdRun(args, modeLocal)
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

// loadConfig reads the -config flag and loads that file. A missing file is fine:
// the defaults are used.
func loadConfig(fs *flag.FlagSet, args []string) (config.Config, error) {
	path := fs.String("config", defaultConfig, "configuration file")
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

// buildDB loads the tracker database: the built-in list of known services plus every
// downloaded block list. A list that fails to load is logged and skipped. In demo mode
// it also adds the demo's made-up tracker domains.
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

type runMode int

const (
	modeService runMode = iota // Linux service: the whole home network
	modeDemo                   // simulated traffic, nothing leaves the machine
	modeLocal                  // experimental: filters only this computer (Windows, macOS, Linux desktop)
)

// localDataDir is where local mode keeps its files: %LOCALAPPDATA%\Vahti on Windows.
func localDataDir() string {
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "Vahti")
	}
	return "vahti-data"
}

// localConfig serves only this computer: DNS and the web interface listen on
// loopback, so nothing is reachable from the network.
func localConfig(dir, webAddr, router string) (config.Config, error) {
	cfg := config.Default()
	cfg.DataDir = dir
	cfg.DNS.Listen = []string{"127.0.0.1:53", "[::1]:53"}
	cfg.DNS.AllowedClients = []string{"127.0.0.0/8", "::1/128"}
	cfg.Web.AllowedClients = []string{"127.0.0.0/8", "::1/128"}
	cfg.Web.Listen = webAddr
	cfg.Web.RedirectListen = ""
	cfg.Web.TLS = false
	if router != "" {
		cfg.DNS.LocalResolver = net.JoinHostPort(router, "53")
	}
	return cfg, cfg.Validate()
}

// cmdRun starts Vahti in the given mode (service, demo or local): it loads the settings
// and saved state, starts the DNS server and the web interface, saves the state regularly,
// reloads changed block lists, reports health to systemd and shuts down cleanly on a signal.
func cmdRun(args []string, mode runMode) error {
	demoMode := mode == modeDemo
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	var cfg config.Config
	var err error
	if mode == modeLocal {
		dir := fs.String("dir", localDataDir(), "data directory")
		webAddr := fs.String("web", "127.0.0.1:8080", "web interface address (this computer only)")
		router := fs.String("router", "", "router IP address for local device names, e.g. 192.168.1.1")
		if err := fs.Parse(args); err != nil {
			return err
		}
		cfg, err = localConfig(*dir, *webAddr, *router)
		if err != nil {
			return err
		}
	} else if demoMode {
		dir := fs.String("dir", "./vahti-demo", "demo data directory")
		webAddr := fs.String("web", "127.0.0.1:8080", "web interface address")
		dnsAddr := fs.String("dns", "127.0.0.1:5353", "DNS address (e.g. dig @127.0.0.1 -p 5353)")
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
		if mode != modeService {
			fmt.Fprintf(os.Stderr, "\n  Open http://%s and enter the setup code: %s\n\n", cfg.Web.Listen, code)
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
		if mode != modeLocal {
			return err
		}
		// IPv6 loopback may be disabled; IPv4 alone still works.
		slog.Warn("could not listen on all addresses; trying IPv4 only", "err", err)
		if err2 := dnsSrv.Start(cfg.DNS.Listen[:1]); err2 != nil {
			return fmt.Errorf("%w\nport 53 is probably used by another program (e.g. Windows Internet Connection Sharing, Hyper-V or another DNS program); check in PowerShell: Get-NetUDPEndpoint -LocalPort 53", err2)
		}
	}
	defer dnsSrv.Shutdown()
	slog.Info("dns listening", "addrs", dnsSrv.Addrs())

	webSrv, err := web.New(web.Config{
		AllowedClients: cfg.Web.AllowedClients, Hostnames: cfg.Web.Hostnames, TLS: cfg.Web.TLS, CertFP: fp,
		SetupCodePath: setupPath, Demo: demoMode, Local: mode == modeLocal, Version: version, Upstreams: cfg.DNS.Upstreams,
		Speed: dnsSrv.Speed,
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
	if mode != modeService {
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

	// Reload blocklists when their files change (Windows has no SIGHUP).
	go func() {
		last := listsModTime(cfg)
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if m := listsModTime(cfg); !m.Equal(last) {
					last = m
					slog.Info("blocklists changed; reloading")
					db.Replace(buildDB(cfg, demoMode))
				}
			}
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
	if mode == modeLocal {
		nameThisComputer(eng)
		fmt.Fprintf(os.Stderr, "\n  Vahti is running. Manage it at http://%s\n  To stop: close this window or press Ctrl+C.\n  If you started Vahti with start-vahti.ps1, DNS settings are restored automatically.\n\n", cfg.Web.Listen)
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

// nameThisComputer gives the loopback clients a friendly name in local mode.
func nameThisComputer(eng *engine.Engine) {
	name, kind := "This computer", "computer"
	for _, ip := range []string{"127.0.0.1", "::1"} {
		id := eng.Identify(netip.MustParseAddr(ip), "")
		if d, ok := eng.Device(id); ok && strings.HasPrefix(d.Name, "Vahti device") {
			n := name
			if ip == "::1" {
				n = name + " (IPv6)"
			}
			_, _ = eng.UpdateDevice(id, engine.DeviceUpdate{Name: &n, Kind: &kind})
		}
	}
}

// listsModTime returns the newest modification time of the block list files, so a
// change can be noticed and the lists reloaded.
func listsModTime(cfg config.Config) time.Time {
	var latest time.Time
	for _, l := range cfg.Lists {
		if fi, err := os.Stat(filepath.Join(cfg.DataDir, "lists", l.Name+".txt")); err == nil && fi.ModTime().After(latest) {
			latest = fi.ModTime()
		}
	}
	return latest
}

// saveState writes the devices, rules and settings to disk if they have changed
// (or always when force is true). If saving fails, the state stays marked as unsaved.
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

// selfCheckAddr turns the DNS listen address into one Vahti can query itself for the
// health check (0.0.0.0 or :: becomes 127.0.0.1).
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

// cmdUpdateLists downloads the block lists named in the configuration. It is the only
// command that fetches anything from the internet. One failed list does not stop the others.
func cmdUpdateLists(args []string) error {
	fs := flag.NewFlagSet("update-lists", flag.ExitOnError)
	local := fs.Bool("local", false, "update the lists for local mode (vahti local)")
	path := fs.String("config", defaultConfig, "configuration file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var cfg config.Config
	var err error
	if *local {
		cfg, err = localConfig(localDataDir(), "127.0.0.1:8080", "")
	} else {
		cfg, _, err = config.Load(*path)
	}
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
	if *local {
		fmt.Println("Done. Vahti picks up the new lists within 30 seconds.")
	} else {
		fmt.Println("Done. Apply the lists with: sudo systemctl reload vahti")
	}
	if failed > 0 {
		return fmt.Errorf("%d list(s) failed", failed)
	}
	return nil
}

// downloadList downloads one block list (max 64 MB), keeps only valid domain names and
// saves them atomically to dest. It returns the number of domains.
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

// cmdResetAdmin forgets the admin password and creates a new setup code, for when the
// password is lost. Vahti must be stopped first.
func cmdResetAdmin(args []string) error {
	fs := flag.NewFlagSet("reset-admin", flag.ExitOnError)
	cfg, err := loadConfig(fs, args)
	if err != nil {
		return err
	}
	lk, err := lock.Acquire(cfg.DataDir)
	if errors.Is(err, lock.ErrBusy) {
		fmt.Fprintln(os.Stderr, "Vahti is running. Stop it first: sudo systemctl stop vahti")
		return err
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
	fmt.Printf("Password removed. New setup code: %s\nStart Vahti: sudo systemctl start vahti\n", code)
	return nil
}
