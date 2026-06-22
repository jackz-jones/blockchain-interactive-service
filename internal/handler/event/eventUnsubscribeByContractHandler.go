package event

import (
	"net/http"

	eventlogic "github.com/jackz-jones/blockchain-interactive-service/internal/logic/http/event"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func UnsubscribeByContractHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UnsubscribeByContractRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := eventlogic.NewUnsubscribeByContractLogic(r.Context(), svcCtx)
		resp, err := l.UnsubscribeByContract(req.ContractConfigID)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
