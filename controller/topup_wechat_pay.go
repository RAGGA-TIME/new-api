package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

type WeChatPayRequest struct {
	Amount int64 `json:"amount"`
}

func getWeChatPayClient() (*core.Client, error) {
	privateKey, err := utils.LoadPrivateKey(setting.WeChatPayPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("加载商户私钥失败: %w", err)
	}

	ctx := context.Background()

	if setting.WeChatPayPublicKeyID != "" && setting.WeChatPayPublicKey != "" {
		publicKey, err := utils.LoadPublicKey(setting.WeChatPayPublicKey)
		if err != nil {
			return nil, fmt.Errorf("加载微信支付公钥失败: %w", err)
		}
		client, err := core.NewClient(
			ctx,
			option.WithWechatPayPublicKeyAuthCipher(
				setting.WeChatPayMchID,
				setting.WeChatPaySerialNo,
				privateKey,
				setting.WeChatPayPublicKeyID,
				publicKey,
			),
		)
		if err != nil {
			return nil, fmt.Errorf("创建微信支付客户端失败(公钥模式): %w", err)
		}
		return client, nil
	}

	client, err := core.NewClient(
		ctx,
		option.WithWechatPayAutoAuthCipher(
			setting.WeChatPayMchID,
			setting.WeChatPaySerialNo,
			privateKey,
			setting.WeChatPayAPIv3Key,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("创建微信支付客户端失败(证书模式): %w", err)
	}
	return client, nil
}

func RequestWeChatPayPay(c *gin.Context) {
	if !isWeChatPayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "管理员未开启微信支付充值"})
		return
	}

	var req WeChatPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	if req.Amount < int64(setting.WeChatPayMinTopUp) {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值金额不能小于 %d 元", setting.WeChatPayMinTopUp)})
		return
	}

	id := c.GetInt("id")

	payMoneyCents := req.Amount * 100 // CNY yuan -> cents
	if payMoneyCents <= 0 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额无效"})
		return
	}

	tradeNo := fmt.Sprintf("WXP%dNO%s%d", id, common.GetRandomString(6), time.Now().Unix())

	callBackAddress := service.GetCallbackAddress()
	notifyUrl := callBackAddress + "/api/wechat-pay/webhook"

	timeExpire := time.Now().Add(5 * time.Minute)

	client, err := getWeChatPayClient()
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付 创建客户端失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付配置错误"})
		return
	}

	svc := native.NativeApiService{Client: client}
	resp, _, err := svc.Prepay(c.Request.Context(), native.PrepayRequest{
		Appid:       core.String(setting.WeChatPayAppID),
		Mchid:       core.String(setting.WeChatPayMchID),
		Description: core.String(fmt.Sprintf("充值 %d 元", req.Amount)),
		OutTradeNo:  core.String(tradeNo),
		TimeExpire:  &timeExpire,
		NotifyUrl:   core.String(notifyUrl),
		Amount: &native.Amount{
			Total:    core.Int64(payMoneyCents),
			Currency: core.String("CNY"),
		},
		SceneInfo: &native.SceneInfo{
			PayerClientIp: core.String(c.ClientIP()),
		},
	})
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付 预下单失败 user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	topUp := &model.TopUp{
		UserId:        id,
		Amount:        req.Amount,
		Money:         float64(req.Amount),
		TradeNo:       tradeNo,
		PaymentMethod: model.PaymentMethodWeChatPay,
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付 创建充值订单失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("微信支付 充值订单创建成功 user_id=%d trade_no=%s amount=%d money=%.2f code_url=%q", id, tradeNo, req.Amount, float64(req.Amount), *resp.CodeUrl))

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"code_url": *resp.CodeUrl,
			"trade_no": tradeNo,
		},
	})
}

func RequestWeChatPayAmount(c *gin.Context) {
	if !isWeChatPayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "管理员未开启微信支付充值"})
		return
	}

	var req WeChatPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	if req.Amount < int64(setting.WeChatPayMinTopUp) {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值金额不能小于 %d 元", setting.WeChatPayMinTopUp)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success", "data": strconv.FormatFloat(float64(req.Amount), 'f', 2, 64)})
}

type WeChatPayNotifyResource struct {
	TradeState     string `json:"trade_state"`
	OutTradeNo     string `json:"out_trade_no"`
	TransactionId  string `json:"transaction_id"`
	TradeStateDesc string `json:"trade_state_desc"`
}

func WeChatPayWebhook(c *gin.Context) {
	ctx := c.Request.Context()
	if !isWeChatPayWebhookEnabled() {
		logger.LogWarn(ctx, fmt.Sprintf("微信支付 webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.Status(http.StatusForbidden)
		return
	}

	var verifierObj auth.Verifier
	if setting.WeChatPayPublicKeyID != "" && setting.WeChatPayPublicKey != "" {
		publicKey, err := utils.LoadPublicKey(setting.WeChatPayPublicKey)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("微信支付 webhook 加载公钥失败 error=%q", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		verifierObj = verifiers.NewSHA256WithRSAPubkeyVerifier(setting.WeChatPayPublicKeyID, *publicKey)
	} else {
		mgr := downloader.MgrInstance()
		certVisitor := mgr.GetCertificateVisitor(setting.WeChatPayMchID)
		verifierObj = verifiers.NewSHA256WithRSAVerifier(certVisitor)
	}

	handler, err := notify.NewRSANotifyHandler(setting.WeChatPayAPIv3Key, verifierObj)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("微信支付 webhook 创建通知处理器失败 error=%q", err.Error()))
		c.Status(http.StatusInternalServerError)
		return
	}

	var content WeChatPayNotifyResource
	_, err = handler.ParseNotifyRequest(ctx, c.Request, &content)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("微信支付 webhook 解析通知失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.Status(http.StatusBadRequest)
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("微信支付 webhook 收到通知 trade_no=%s trade_state=%s transaction_id=%s client_ip=%s", content.OutTradeNo, content.TradeState, content.TransactionId, c.ClientIP()))

	if content.TradeState == "SUCCESS" {
		LockOrder(content.OutTradeNo)
		defer UnlockOrder(content.OutTradeNo)

		callerIp := c.ClientIP()
		if err := model.RechargeWeChatPay(content.OutTradeNo, callerIp); err != nil {
			logger.LogError(ctx, fmt.Sprintf("微信支付 充值处理失败 trade_no=%s client_ip=%s error=%q", content.OutTradeNo, callerIp, err.Error()))
		} else {
			logger.LogInfo(ctx, fmt.Sprintf("微信支付 充值成功 trade_no=%s client_ip=%s", content.OutTradeNo, callerIp))
		}
	}

	c.Status(http.StatusNoContent)
}

func GetWeChatPayOrderStatus(c *gin.Context) {
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
		expireTime := topUp.CreateTime + 300 // 5 minutes
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
