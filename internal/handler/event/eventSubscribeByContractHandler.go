package event

import (
	"net/http"

	eventlogic "github.com/jackz-jones/blockchain-interactive-service/internal/logic/http/event" //nolint:staticcheck // 过渡期：事件轮询已迁移 gRPC 流式接口，routes.go 清理后整批删除
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func SubscribeByContractHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SubscribeByContractRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := eventlogic.NewSubscribeByContractLogic(r.Context(), svcCtx)
		resp, err := l.SubscribeByContract(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

func ListAvailableContractsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := eventlogic.NewSubscribeByContractLogic(r.Context(), svcCtx)
		resp, err := l.ListAvailableContracts()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
