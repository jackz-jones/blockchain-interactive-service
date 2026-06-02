package gateway

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

// ========== 事件订阅 API ==========

// subscriptionStore 存储订阅信息（内存管理，生产环境可改为 Redis/DB）
var (
	subscriptionStore   = sync.Map{} // subscriptionID -> *SubscriptionInfo
	subscriptionCounter uint64
	subscriptionMu      sync.Mutex
)

// SubscriptionInfo 订阅信息
type SubscriptionInfo struct {
	ID               string `json:"id"`
	TenantID         uint   `json:"tenant_id"`
	ChainConfigID    uint   `json:"chain_config_id"`
	ContractConfigID uint   `json:"contract_config_id"`
	ChainName        string `json:"chain_name"`
	ContractName     string `json:"contract_name"`
	GroupName        string `json:"group_name"`
	ConsumerName     string `json:"consumer_name"`
	CreatedAt        string `json:"created_at"`
}

// EventSubscribeRequest 事件订阅请求体
type EventSubscribeRequest struct {
	ChainName    string `json:"chain_name"`    // 链名称（必填）
	ContractName string `json:"contract_name"` // 合约名称（与 contract_addr 二选一）
	ContractAddr string `json:"contract_addr"` // 合约地址（与 contract_name 二选一）
}

// EventPollRequest 事件轮询请求参数
type EventPollRequest struct {
	SubscriptionID string `form:"subscription_id"` // 订阅 ID
	Count          int    `form:"count"`           // 拉取数量（默认 10）
}

// EventSubscribeHandler 创建事件订阅
// POST /api/v1/events/subscribe
func EventSubscribeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req EventSubscribeRequest
		if err := httpx.ParseJsonBody(r, &req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if req.ChainName == "" {
			errorResponse(w, http.StatusBadRequest, "chain_name is required")
			return
		}
		if req.ContractName == "" && req.ContractAddr == "" {
			errorResponse(w, http.StatusBadRequest, "contract_name or contract_addr is required")
			return
		}

		// 通过映射层定位到 DB 记录
		result, err := svcCtx.ConfigResolver.Resolve(r.Context(), tenantID, req.ChainName, req.ContractName, req.ContractAddr)
		if err != nil {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}

		// 生成订阅 ID
		subscriptionMu.Lock()
		subscriptionCounter++
		subID := fmt.Sprintf("sub_%d_%d_%d", tenantID, result.ContractConfigID, subscriptionCounter)
		subscriptionMu.Unlock()

		// 创建消费者组信息
		groupName := fmt.Sprintf("group_%s", subID)
		consumerName := fmt.Sprintf("consumer_%s", subID)

		// 存储订阅信息
		subInfo := &SubscriptionInfo{
			ID:               subID,
			TenantID:         tenantID,
			ChainConfigID:    result.ChainConfigID,
			ContractConfigID: result.ContractConfigID,
			ChainName:        result.ChainName,
			ContractName:     result.ContractName,
			GroupName:        groupName,
			ConsumerName:     consumerName,
			CreatedAt:        time.Now().Format("2006-01-02T15:04:05Z07:00"),
		}
		subscriptionStore.Store(subID, subInfo)

		successResponse(w, map[string]interface{}{
			"subscription_id":    subID,
			"chain_config_id":    result.ChainConfigID,
			"contract_config_id": result.ContractConfigID,
			"chain_name":         result.ChainName,
			"contract_name":      result.ContractName,
		})
	}
}

// EventPollHandler 轮询事件
// GET /api/v1/events/poll?subscription_id=xxx&count=10
func EventPollHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		subscriptionID := r.URL.Query().Get("subscription_id")
		if subscriptionID == "" {
			errorResponse(w, http.StatusBadRequest, "subscription_id is required")
			return
		}

		// 查找订阅信息
		subVal, ok := subscriptionStore.Load(subscriptionID)
		if !ok {
			errorResponse(w, http.StatusNotFound, "subscription not found")
			return
		}
		subInfo := subVal.(*SubscriptionInfo)

		// 校验租户归属
		if subInfo.TenantID != tenantID {
			errorResponse(w, http.StatusForbidden, "subscription not owned by current tenant")
			return
		}

		// 从 Redis stream 读取事件
		var events []commonEvent.CrossChainEvent
		err := svcCtx.RedisClient.SubscribeCrossChainEventFromStream(
			r.Context(),
			subInfo.ChainConfigID,
			subInfo.ContractConfigID,
			subInfo.GroupName,
			subInfo.ConsumerName,
			func(event commonEvent.CrossChainEvent) error {
				events = append(events, event)
				return nil
			},
			false,                // wantTrimOldMsg
			10,                   // ackCountThreshold
			100*time.Millisecond, // block（短阻塞，快速返回）
		)
		if err != nil {
			// SubscribeCrossChainEventFromStream 可能因为 stream 不存在而报错，返回空事件列表
			svcCtx.Logger.Errorf("poll events error (may be no stream yet): %v", err)
		}

		successResponse(w, map[string]interface{}{
			"subscription_id": subscriptionID,
			"events":          events,
			"count":           len(events),
		})
	}
}

// EventUnsubscribeHandler 取消事件订阅
// DELETE /api/v1/events/subscribe/:subscriptionId
func EventUnsubscribeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		vars := pathvar.Vars(r)
		subscriptionID := vars["subscriptionId"]
		if subscriptionID == "" {
			errorResponse(w, http.StatusBadRequest, "subscription_id is required")
			return
		}

		// 查找订阅信息
		subVal, ok := subscriptionStore.Load(subscriptionID)
		if !ok {
			errorResponse(w, http.StatusNotFound, "subscription not found")
			return
		}
		subInfo := subVal.(*SubscriptionInfo)

		// 校验租户归属
		if subInfo.TenantID != tenantID {
			errorResponse(w, http.StatusForbidden, "subscription not owned by current tenant")
			return
		}

		// 清理订阅
		subscriptionStore.Delete(subscriptionID)

		successResponse(w, map[string]interface{}{
			"message":         "subscription deleted",
			"subscription_id": subscriptionID,
		})
	}
}
