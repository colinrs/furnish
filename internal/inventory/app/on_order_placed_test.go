package app_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/colinrs/furnish/internal/inventory/adapter/memory"
	"github.com/colinrs/furnish/internal/inventory/app"
	"github.com/colinrs/furnish/internal/inventory/domain"
	orderingcontract "github.com/colinrs/furnish/internal/ordering/contract"
)

func TestReservesOnceEvenIfDeliveredTwice(t *testing.T) {
	stocks := memory.NewStocks(domain.NewStock("SOFA-OAT-3S", 5))
	r := &app.Reservations{Stocks: stocks, Processed: memory.NewProcessed(), Tx: memory.NoTx{}, Shortage: memory.LogShortage{}}
	payload, _ := json.Marshal(orderingcontract.OrderPlacedV1{
		OrderID: "SO-1", Lines: []orderingcontract.PlacedLineV1{{ProductID: "SOFA-OAT-3S", Quantity: 2}},
	})
	ctx := context.Background()
	for i := 0; i < 2; i++ { // 模拟 outbox 重复投递
		if err := r.OnOrderPlaced(ctx, 42, payload); err != nil {
			t.Fatal(err)
		}
	}
	st, _ := stocks.Get(ctx, "SOFA-OAT-3S")
	if st.Reserved() != 2 {
		t.Fatalf("reserved = %d, want 2", st.Reserved())
	}
}
