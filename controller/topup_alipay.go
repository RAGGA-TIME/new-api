package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/smartwalle/alipay/v3"
)

type AliPayRequest struct {
	Amount int64 `json:"amount"`
}

func getAliPayMoney(amount int64, group string) float64 {
	dAmount := decimal.NewFromInt(amount)
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		dAmount = dAmount.Div(dQuotaPerUnit)
	}

	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}

	dTopupGroupRatio := decimal.NewFromFloat(topupGroupRatio)
	dUnitPrice := decimal.NewFromFloat(setting.AliPayUnitPrice)

	discount := 1.0
	if ds, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(amount)]; ok {
		if ds > 0 {
			discount = ds
		}
	}
	dDiscount := decimal.NewFromFloat(discount)

	payMoney := dAmount.Mul(dUnitPrice).Mul(dTopupGroupRatio).Mul(dDiscount)
	return payMoney.InexactFloat64()
}

func getAliPayClient() (*alipay.Client, error) {
	appID := setting.AliPayAppID
	privateKey := setting.AliPayPrivateKey
	publicKey := setting.AliPayPublicKey

	if appID == "" || privateKey == "" || publicKey == "" {
		return nil, fmt.Errorf("支付宝配置不完整")
	}

	client, err := alipay.New(appID, privateKey, true)
	if err != nil {
		return nil, fmt.Errorf("创建支付宝客户端失败: %w", err)
	}

	if err := client.LoadAliPayPublicKey(publicKey); err != nil {
		return nil, fmt.Errorf("加载支付宝公钥失败: %w", err)
	}
	fmt.Print(client, "client")
	fmt.Print(setting.AliPayAppID)
	return client, nil
}

func RequestAliPayPay(c *gin.Context) {
	if !isAliPayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "管理员未开启支付宝支付充值"})
		return
	}

	var req AliPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	if req.Amount < int64(setting.AliPayMinTopUp) {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值金额不能小于 %d 元", setting.AliPayMinTopUp)})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	payMoney := getAliPayMoney(req.Amount, group)
	if payMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	payMoneyCents := int64(payMoney * 100) // CNY yuan -> cents
	if payMoneyCents <= 0 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额无效"})
		return
	}

	tradeNo := fmt.Sprintf("ALP%dNO%s%d", id, common.GetRandomString(6), time.Now().Unix())

	callBackAddress := service.GetCallbackAddress()
	notifyUrl := callBackAddress + "/api/alipay/webhook"

	client, err := getAliPayClient()
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝 创建客户端失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付配置错误"})
		return
	}

	p := alipay.TradePagePay{}
	p.OutTradeNo = tradeNo
	p.Subject = fmt.Sprintf("充值 %d 元", req.Amount)
	p.TotalAmount = strconv.FormatFloat(payMoney, 'f', 2, 64)
	p.ProductCode = "FAST_INSTANT_TRADE_PAY"
	p.NotifyURL = notifyUrl

	payUrl, err := client.TradePagePay(p)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝 预下单失败 user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	topUp := &model.TopUp{
		UserId:          id,
		Amount:          req.Amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodAliPay,
		PaymentProvider: model.PaymentProviderAliPay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝 创建充值订单失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("支付宝 充值订单创建成功 user_id=%d trade_no=%s amount=%d money=%.2f pay_url=%q", id, tradeNo, req.Amount, payMoney, payUrl.String()))

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"pay_url":  payUrl.String(),
			"trade_no": tradeNo,
		},
	})
}

func RequestAliPayAmount(c *gin.Context) {
	if !isAliPayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "管理员未开启支付宝支付充值"})
		return
	}

	var req AliPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	if req.Amount < int64(setting.AliPayMinTopUp) {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值金额不能小于 %d 元", setting.AliPayMinTopUp)})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	payMoney := getAliPayMoney(req.Amount, group)
	if payMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	result := strconv.FormatFloat(payMoney, 'f', 2, 64)
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("支付宝 询价 amount=%d result=%q", req.Amount, result))
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": result})
}

func AliPayWebhook(c *gin.Context) {
	ctx := c.Request.Context()
	if !isAliPayWebhookEnabled() {
		logger.LogWarn(ctx, fmt.Sprintf("支付宝 webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.Status(http.StatusForbidden)
		return
	}

	client, err := getAliPayClient()
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("支付宝 webhook 创建客户端失败 error=%q", err.Error()))
		c.Status(http.StatusInternalServerError)
		return
	}

	notif, err := client.GetTradeNotification(c.Request)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("支付宝 webhook 验签失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.Status(http.StatusBadRequest)
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("支付宝 webhook 收到通知 trade_no=%s trade_status=%s out_trade_no=%s client_ip=%s", notif.TradeNo, notif.TradeStatus, notif.OutTradeNo, c.ClientIP()))

	if notif.TradeStatus == alipay.TradeStatusSuccess || notif.TradeStatus == alipay.TradeStatusFinished {
		LockOrder(notif.OutTradeNo)
		defer UnlockOrder(notif.OutTradeNo)

		callerIp := c.ClientIP()
		if err := model.RechargeAliPay(notif.OutTradeNo, callerIp); err != nil {
			logger.LogError(ctx, fmt.Sprintf("支付宝 充值处理失败 trade_no=%s client_ip=%s error=%q", notif.OutTradeNo, callerIp, err.Error()))
		} else {
			logger.LogInfo(ctx, fmt.Sprintf("支付宝 充值成功 trade_no=%s client_ip=%s", notif.OutTradeNo, callerIp))
		}
	}

	// 支付宝要求返回 success 字符串
	c.String(http.StatusOK, "success")
}

func GetAliPayOrderStatus(c *gin.Context) {
	tradeNo := c.Query("trade_no")
	if tradeNo == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "缺少订单号"})
		return
	}

	userId := c.GetInt("id")
	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "订单不存在"})
		return
	}
	if topUp.UserId != userId {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "订单不存在"})
		return
	}

	status := topUp.Status
	if status == common.TopUpStatusPending {
		expireTime := topUp.CreateTime + 900 // 15 minutes
		if common.GetTimestamp() > expireTime {
			status = common.TopUpStatusExpired
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"status":   status,
			"trade_no": tradeNo,
			"amount":   topUp.Amount,
			"money":    topUp.Money,
		},
	})
}
