package chainconfig

import (
	"net/http"

	"github.com/jackz-jones/blockchain-interactive-service/internal/logic/http/chainconfig"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func CreateChainConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateChainConfigRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := chainconfig.NewCreateChainConfigLogic(r.Context(), svcCtx)
		resp, err := l.CreateChainConfig(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
