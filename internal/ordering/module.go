// Package ordering 是"下单"限界上下文的组装入口。
// 外部(cmd/api)只通过 New 和 Register 使用它;其他上下文只能依赖 ordering/contract。
package ordering

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/colinrs/furnish/internal/ordering/adapter/catalogacl"
	orderinghttp "github.com/colinrs/furnish/internal/ordering/adapter/http"
	"github.com/colinrs/furnish/internal/ordering/adapter/postgres"
	orderingredis "github.com/colinrs/furnish/internal/ordering/adapter/redis"
	"github.com/colinrs/furnish/internal/ordering/app"
	"github.com/colinrs/furnish/internal/ordering/domain"
)

type Deps struct {
	DB       *gorm.DB
	Redis    *goredis.Client
	ERP      catalogacl.LegacyERP
	ToMATNR  func(domain.ProductID) string
	Credit   app.CreditPolicy
	Tx       app.Transactor
	Recorder app.EventRecorder
}

type Module struct{ svc *app.Service }

func New(d Deps) *Module {
	return &Module{svc: &app.Service{
		Orders:  postgres.NewRepository(d.DB),
		Catalog: catalogacl.New(d.ERP, d.ToMATNR),
		Credit:  d.Credit,
		Tx:      d.Tx,
		Events:  d.Recorder,
		Idem:    orderingredis.NewIdempotencyGuard(d.Redis),
		NewID:   newOrderID,
		Now:     time.Now,
	}}
}

func (m *Module) Register(mux *http.ServeMux) { orderinghttp.NewHandler(m.svc).Register(mux) }

func newOrderID() domain.OrderID {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return domain.OrderID("SO-" + time.Now().Format("20060102") + "-" + hex.EncodeToString(b))
}
