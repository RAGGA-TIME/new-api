package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/shopspring/decimal"
	"github.com/smartwalle/alipay/v3"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	stripeRefund "github.com/stripe/stripe-go/v81/refund"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/refunddomestic"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

// PaymentRefundResult 退款结果
type PaymentRefundResult struct {
	// 退款是否成功受理
	Success bool
	// 支付渠道的退款单号
	RefundID string
	// 错误信息
	Error string
}

// ProcessPaymentRefund 处理支付渠道退款
// 根据 paymentMethod/paymentProvider 调用对应的退款API
func ProcessPaymentRefund(tradeNo string, paymentMethod string, paymentProvider string, amount int64, money float64) (*PaymentRefundResult, error) {
	switch paymentMethod {
	case model.PaymentMethodWeChatPay:
		// 微信支付：amount 是人民币元，退款需转为分
		totalCents := amount * 100
		if totalCents <= 0 {
			return nil, fmt.Errorf("微信支付退款金额无效: %d", amount)
		}
		result, err := refundWeChatPay(tradeNo, totalCents, totalCents)
		if err != nil {
			common.SysError(fmt.Sprintf("微信支付退款异常 trade_no=%s error=%q", tradeNo, err.Error()))
			return nil, err
		}
		return result, nil

	case model.PaymentMethodAliPay:
		// 支付宝：amount 是人民币元
		refundAmount := decimal.NewFromInt(amount).StringFixed(2)
		result, err := refundAliPay(tradeNo, refundAmount)
		if err != nil {
			common.SysError(fmt.Sprintf("支付宝退款异常 trade_no=%s error=%q", tradeNo, err.Error()))
			return nil, err
		}
		return result, nil

	default:
		// 根据支付提供商判断
		switch paymentProvider {
		case model.PaymentProviderStripe:
			result, err := refundStripe(tradeNo)
			if err != nil {
				common.SysError(fmt.Sprintf("Stripe退款异常 trade_no=%s error=%q", tradeNo, err.Error()))
				return nil, err
			}
			return result, nil

		case model.PaymentProviderEpay:
			return &PaymentRefundResult{
				Success: false,
				Error:   "易支付不支持在线退款，请联系管理员手动处理",
			}, nil

		case model.PaymentProviderCreem:
			return &PaymentRefundResult{
				Success: false,
				Error:   "Creem不支持在线退款，请联系管理员手动处理",
			}, nil

		case model.PaymentProviderWaffo, model.PaymentProviderWaffoPancake:
			return &PaymentRefundResult{
				Success: false,
				Error:   "Waffo不支持在线退款，请联系管理员手动处理",
			}, nil

		default:
			return &PaymentRefundResult{
				Success: false,
				Error:   fmt.Sprintf("不支持的支付方式退款: %s/%s", paymentMethod, paymentProvider),
			}, nil
		}
	}
}

// refundWeChatPay 调用微信支付退款API
func refundWeChatPay(tradeNo string, refundAmountCents int64, totalAmountCents int64) (*PaymentRefundResult, error) {
	client, err := createWeChatPayClient()
	if err != nil {
		return nil, fmt.Errorf("创建微信支付客户端失败: %w", err)
	}

	svc := &refunddomestic.RefundsApiService{Client: client}

	outRefundNo := fmt.Sprintf("RF%d%s", time.Now().Unix(), tradeNo[:min(8, len(tradeNo))])

	resp, _, err := svc.Create(context.Background(), refunddomestic.CreateRequest{
		OutTradeNo:  core.String(tradeNo),
		OutRefundNo: core.String(outRefundNo),
		Reason:      core.String("用户申请退款"),
		Amount: &refunddomestic.AmountReq{
			Refund:   core.Int64(2),
			Total:    core.Int64(2),
			Currency: core.String("CNY"),
		},
	})
	if err != nil {
		return &PaymentRefundResult{
			Success: false,
			Error:   fmt.Sprintf("微信支付退款失败: %s", err.Error()),
		}, nil
	}

	refundID := ""
	if resp.RefundId != nil {
		refundID = *resp.RefundId
	}

	return &PaymentRefundResult{
		Success:  true,
		RefundID: refundID,
	}, nil
}

// refundAliPay 调用支付宝退款API
func refundAliPay(tradeNo string, refundAmount string) (*PaymentRefundResult, error) {
	client, err := createAliPayClient()
	if err != nil {
		return nil, fmt.Errorf("创建支付宝客户端失败: %w", err)
	}

	p := alipay.TradeRefund{}
	p.OutTradeNo = tradeNo
	p.RefundAmount = refundAmount
	p.OutRequestNo = fmt.Sprintf("RF%d%s", time.Now().Unix(), tradeNo[:min(8, len(tradeNo))])

	resp, err := client.TradeRefund(context.Background(), p)
	if err != nil {
		return &PaymentRefundResult{
			Success: false,
			Error:   fmt.Sprintf("支付宝退款失败: %s", err.Error()),
		}, nil
	}

	if resp.Code != alipay.CodeSuccess {
		return &PaymentRefundResult{
			Success: false,
			Error:   fmt.Sprintf("支付宝退款失败: %s - %s", resp.Code, resp.Msg),
		}, nil
	}

	return &PaymentRefundResult{
		Success:  true,
		RefundID: resp.TradeNo,
	}, nil
}

// refundStripe 调用Stripe退款API
// Stripe 退款需要通过 PaymentIntent ID，因此需要先通过 Checkout Session 获取
func refundStripe(tradeNo string) (*PaymentRefundResult, error) {
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		return nil, fmt.Errorf("无效的Stripe API密钥")
	}

	stripe.Key = setting.StripeApiSecret

	// 通过 client_reference_id 查找 checkout session
	params := &stripe.CheckoutSessionListParams{}
	params.Limit = stripe.Int64(100)
	iter := session.List(params)
	var targetSession *stripe.CheckoutSession

	for iter.Next() {
		s := iter.CheckoutSession()
		if s.ClientReferenceID == tradeNo {
			targetSession = s
			break
		}
	}
	if iter.Err() != nil {
		return &PaymentRefundResult{
			Success: false,
			Error:   fmt.Sprintf("Stripe 查找Checkout Session失败: %s", iter.Err().Error()),
		}, nil
	}

	if targetSession == nil {
		return &PaymentRefundResult{
			Success: false,
			Error:   "Stripe 未找到对应的Checkout Session，无法退款",
		}, nil
	}

	if targetSession.PaymentIntent == nil {
		return &PaymentRefundResult{
			Success: false,
			Error:   "Stripe Checkout Session无关联的PaymentIntent，无法退款",
		}, nil
	}

	// 通过 PaymentIntent 发起退款
	refundParams := &stripe.RefundParams{
		PaymentIntent: stripe.String(targetSession.PaymentIntent.ID),
		Reason:        stripe.String(string(stripe.RefundReasonRequestedByCustomer)),
	}

	r, err := stripeRefund.New(refundParams)
	if err != nil {
		return &PaymentRefundResult{
			Success: false,
			Error:   fmt.Sprintf("Stripe 退款失败: %s", err.Error()),
		}, nil
	}

	return &PaymentRefundResult{
		Success:  true,
		RefundID: r.ID,
	}, nil
}

// createWeChatPayClient 创建微信支付客户端
func createWeChatPayClient() (*core.Client, error) {
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

// createAliPayClient 创建支付宝客户端
func createAliPayClient() (*alipay.Client, error) {
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

	return client, nil
}
