package dashboard

import (
	"net/http"

	"github.com/jackz-jones/blockchain-interactive-service/internal/logic/http/dashboard"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetRealtimeCostHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := dashboard.NewGetRealtimeCostLogic(r.Context(), svcCtx)
		resp, err := l.GetRealtimeCost()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
