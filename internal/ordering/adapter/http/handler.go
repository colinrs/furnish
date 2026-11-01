// Package http 是入站适配器:把 HTTP 请求翻译成应用层命令,把领域错误翻译成状态码。
package http

import (
	"encoding/json"
	"errors"
	nethttp "net/http"
	"strconv"

	"github.com/colinrs/furnish/internal/ordering/app"
	"github.com/colinrs/furnish/internal/ordering/domain"
)

type Handler struct{ svc *app.Service }

func NewHandler(svc *app.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *nethttp.ServeMux) {
	mux.HandleFunc("POST /orders", h.placeOrder)
	mux.HandleFunc("PUT /orders/{id}/lines/{lineNo}/quantity", h.changeQuantity)
	mux.HandleFunc("POST /orders/{id}/lock", h.lock)
}

type placeOrderReq struct {
	CustomerID string `json:"customer_id"`
	Items      []struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	} `json:"items"`
}

func (h *Handler) placeOrder(w nethttp.ResponseWriter, r *nethttp.Request) {
	var req placeOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, nethttp.StatusBadRequest, "invalid JSON body")
		return
	}
	cmd := app.PlaceOrderCmd{IdempotencyKey: r.Header.Get("Idempotency-Key"), CustomerID: req.CustomerID}
	for _, it := range req.Items {
		cmd.Items = append(cmd.Items, app.ItemCmd{ProductID: it.ProductID, Quantity: it.Quantity})
	}
	id, err := h.svc.PlaceOrder(r.Context(), cmd)
	if err != nil {
		writeDomainErr(w, err)
		return
	}
	w.WriteHeader(nethttp.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"order_id": string(id)})
}

func (h *Handler) changeQuantity(w nethttp.ResponseWriter, r *nethttp.Request) {
	lineNo, err := strconv.Atoi(r.PathValue("lineNo"))
	if err != nil {
		writeErr(w, nethttp.StatusBadRequest, "lineNo must be a number")
		return
	}
	var body struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, nethttp.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.svc.ChangeQuantity(r.Context(), r.PathValue("id"), lineNo, body.Quantity); err != nil {
		writeDomainErr(w, err)
		return
	}
	w.WriteHeader(nethttp.StatusNoContent)
}

func (h *Handler) lock(w nethttp.ResponseWriter, r *nethttp.Request) {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := h.svc.Lock(r.Context(), r.PathValue("id"), body.Reason); err != nil {
		writeDomainErr(w, err)
		return
	}
	w.WriteHeader(nethttp.StatusNoContent)
}

func writeDomainErr(w nethttp.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrOrderNotFound), errors.Is(err, domain.ErrOrderLineNotFound):
		writeErr(w, nethttp.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrConcurrentModification), errors.Is(err, app.ErrDuplicateRequest):
		writeErr(w, nethttp.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrEmptyOrder), errors.Is(err, domain.ErrInvalidQuantity),
		errors.Is(err, domain.ErrCreditLimitExceeded), errors.Is(err, domain.ErrOrderLocked):
		writeErr(w, nethttp.StatusUnprocessableEntity, err.Error())
	default:
		writeErr(w, nethttp.StatusInternalServerError, "internal error")
	}
}

func writeErr(w nethttp.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
