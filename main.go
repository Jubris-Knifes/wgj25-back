package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"database/sql"

	"github.com/Jubris-Knifes/wgj25-back/config"
	"github.com/Jubris-Knifes/wgj25-back/repository"
	"github.com/Jubris-Knifes/wgj25-back/service"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/olahol/melody"
	zrokEnvironment "github.com/openziti/zrok/environment"
	zrok "github.com/openziti/zrok/sdk/golang/sdk"
	_ "modernc.org/sqlite"
)

var logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, Level: slog.LevelDebug}))

func main() {
	endChan := make(chan os.Signal, 1)
	signal.Notify(endChan, syscall.SIGTERM, syscall.SIGINT)

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	runMigrations(db)

	m := melody.New()
	repo := repository.New(logger, db)
	svc := service.New(logger, repo, m)

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logger.Info("AAAHHHHH", "headers", r.Header)
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// CORS headers

		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		reqHeaders := r.Header.Get("Access-Control-Request-Headers")
		if reqHeaders != "" {
			w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
		} else {
			w.Header().Set("Access-Control-Allow-Headers", "Host, User-Agent, Accept, Accept-Language, Accept-Encoding, Sec-WebSocket-Version, Origin, Sec-WebSocket-Extensions, Sec-WebSocket-Key, Sec-GPC, Connection, Sec-Fetch-Dest, Sec-Fetch-Mode, Sec-Fetch-Site, Pragma, Cache-Control, Upgrade, ")
		}
		w.Header().Set("Access-Control-Expose-Headers", "Content-Type, Authorization, Connection, Upgrade, Sec-Websocket-Version, Sec-Websocket-Key, Sec-WebSocket-Extensions, Sec-WebSocket-Protocol")
		w.Header().Set("Vary", "Origin")
		// CSP header (very permissive, adjust as needed)
		w.Header().Set("Content-Security-Policy", "default-src 'self' ws: wss: 'unsafe-inline' data: gap: ; script-src *; connect-src ws: wss: ; img-src *; style-src *;")

		// Handle preflight OPTIONS request
		if r.Method == "OPTIONS" {
			logger.Info("Preflight request received", "origin", origin, "headers", r.Header)
			w.WriteHeader(http.StatusOK)
			return
		}
		logger.Info("New connection", "remote_address", r.RemoteAddr, "headers", r.Header, "url", r.URL)

		m.HandleRequest(w, r)
	})

	m.Upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	m.HandleConnect(func(s *melody.Session) {
		logger.Info("New connection established",
			"remote_address", s.RemoteAddr().String(),
			"keys", s.Keys,
		)
		svc.NewConnection(s)
	})
	m.HandleDisconnect(func(s *melody.Session) {
		logger.Info("Session disconnected",
			"remote_address", s.RemoteAddr().String(),
			"keys", s.Keys,
		)
		svc.ClosedConnection(s)
	})
	m.HandleMessage(func(s *melody.Session, data []byte) {
		logger.Info("New message received",
			"remote_address", s.RemoteAddr().String(),
			"keys", s.Keys,
		)
		svc.HandleMessage(s, data)
	})
	m.HandleMessageBinary(func(s *melody.Session, data []byte) {
		logger.Info("New message binary received",
			"remote_address", s.RemoteAddr().String(),
			"keys", s.Keys,
		)
		svc.HandleMessage(s, data)
	})

	port := config.Get().Port

	go func() { panic(http.ListenAndServe(fmt.Sprintf(":%d", port), mux)) }()

	root, err := zrokEnvironment.LoadRoot()
	if err != nil {
		panic(err)
	}

	shareToken := config.Get().Zrok.ReservedName
	FrontendEndpoint := fmt.Sprintf("wss://%s.share.zrok.io", shareToken)

	if !config.Get().Zrok.UseReserved || config.Get().Zrok.ReservedName == "" {
		zrokRequest := &zrok.ShareRequest{
			BackendMode: zrok.ProxyBackendMode,
			ShareMode:   zrok.PublicShareMode,
			Frontends:   []string{"public"},
			Target:      fmt.Sprintf("http://localhost:%d", port),
		}

		var err error

		shr, err := zrok.CreateShare(root, zrokRequest)
		if err != nil {
			panic(err)
		}

		defer zrok.DeleteShare(root, shr)

		shareToken = shr.Token
		FrontendEndpoint = fmt.Sprintf("wss://%s", strings.TrimPrefix(shr.FrontendEndpoints[0], "https://"))
	}

	conn, err := zrok.NewListener(shareToken, root)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	go func() {
		if err := http.Serve(conn, mux); err != nil {
			logger.Error("listener closed", "error", err)
		}
	}()

	logger.Info("Share created", "frontend_endpoints", FrontendEndpoint)

	<-endChan
}

func runMigrations(db *sql.DB) {
	driver, err := sqlite.WithInstance(db, &sqlite.Config{})
	if err != nil {
		panic(err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"sqlite", driver,
	)
	if err != nil {
		panic(err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		panic(err)
	}

}
