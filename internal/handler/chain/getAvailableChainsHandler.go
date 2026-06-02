package chain

import (
	"net/http"

	"github.com/jackz-jones/blockchain-interactive-service/internal/logic/http/chain"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetAvailableChainsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := chain.NewGetAvailableChainsLogic(r.Context(), svcCtx)
		resp, err := l.GetAvailableChains()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
