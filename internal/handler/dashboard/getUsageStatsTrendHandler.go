package dashboard

import (
	"net/http"

	dashboardlogic "github.com/jackz-jones/blockchain-interactive-service/internal/logic/http/dashboard"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetUsageStatsTrendHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := dashboardlogic.NewGetUsageStatsTrendLogic(r.Context(), svcCtx)
		l.R = r
		resp, err := l.GetUsageStatsTrend()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
