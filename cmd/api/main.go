// cmd/api 是组装根(composition root):创建基础设施,把各上下文接起来。
// 业务代码不在这里。
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/colinrs/furnish/internal/inventory/adapter/memory"
	inventoryapp "github.com/colinrs/furnish/internal/inventory/app"
	inventorydomain "github.com/colinrs/furnish/internal/inventory/domain"
	"github.com/colinrs/furnish/internal/ordering"
	"github.com/colinrs/furnish/internal/ordering/adapter/catalogacl"
	"github.com/colinrs/furnish/internal/ordering/adapter/credit"
	"github.com/colinrs/furnish/internal/ordering/contract"
	"github.com/colinrs/furnish/internal/ordering/domain"
	"github.com/colinrs/furnish/internal/platform/outbox"
	"github.com/colinrs/furnish/internal/platform/tx"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	db, err := gorm.Open(postgres.Open(env("DATABASE_URL", "postgres://furnish:furnish@localhost:5432/furnish?sslmode=disable")), &gorm.Config{})
	if err != nil {
		slog.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: env("REDIS_ADDR", "localhost:6379")})

	transactor := tx.NewTransactor(db)
	recorder := outbox.NewRecorder(db)

	// 下单上下文
	orderingModule := ordering.New(ordering.Deps{
		DB: db, Redis: rdb, Tx: transactor, Recorder: recorder,
		ERP: catalogacl.StaticERP{
			"000000000010023": {MATNR: "000000000010023", NETPR: "2999.00", WAERS: "CNY", MSTAE: "01"},
		},
		ToMATNR: func(domain.ProductID) string { return "000000000010023" },
		Credit:  credit.Fixed{Limit: domain.MustMoney(5_000_000, "CNY")},
	})

	// 库存上下文:通过 outbox 订阅下单事件(最终一致)
	reservations := &inventoryapp.Reservations{
		Stocks:    memory.NewStocks(inventorydomain.NewStock("SOFA-OAT-3S", 10)),
		Processed: memory.NewProcessed(),
		Tx:        memory.NoTx{},
		Shortage:  memory.LogShortage{},
	}
	relay := outbox.NewRelay(db)
	relay.Subscribe(contract.OrderPlacedV1Name, reservations.OnOrderPlaced)
	go relay.Run(ctx, time.Second)

	mux := http.NewServeMux()
	orderingModule.Register(mux)

	srv := &http.Server{Addr: env("ADDR", ":8080"), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	slog.Info("listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
