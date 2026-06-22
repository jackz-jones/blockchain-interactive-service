package event

import (
	"net/http"

	eventlogic "github.com/jackz-jones/blockchain-interactive-service/internal/logic/http/event"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func RecentEventsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 从 path 参数中获取 contractConfigId
		var req eventlogic.RecentEventsRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := eventlogic.NewRecentEventsLogic(r.Context(), svcCtx)
		resp, err := l.GetRecentEvents(req.ContractConfigID)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
