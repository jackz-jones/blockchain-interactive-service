package auth

import (
	"net/http"

	"github.com/jackz-jones/blockchain-interactive-service/internal/logic/http/auth"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ValidateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ValidateAPIKeyRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := auth.NewValidateLogic(r.Context(), svcCtx)
		resp, err := l.Validate(&req)
		result := types.NewCommonResponse(resp, err)
		httpx.OkJsonCtx(r.Context(), w, result)
	}
}
