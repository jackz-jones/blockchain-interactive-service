package chainconfig

import (
	"net/http"

	"github.com/jackz-jones/blockchain-interactive-service/internal/logic/http/chainconfig"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ListChainConfigsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := chainconfig.NewListChainConfigsLogic(r.Context(), svcCtx)
		resp, err := l.ListChainConfigs()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
