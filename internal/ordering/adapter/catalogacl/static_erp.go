package catalogacl

import (
	"context"
	"fmt"
)

// StaticERP 是演示用的假 ERP,本地跑通流程时使用。
type StaticERP map[string]Material

func (s StaticERP) GetMaterial(_ context.Context, matnr string) (Material, error) {
	m, ok := s[matnr]
	if !ok {
		return Material{}, fmt.Errorf("material %s not found", matnr)
	}
	return m, nil
}
