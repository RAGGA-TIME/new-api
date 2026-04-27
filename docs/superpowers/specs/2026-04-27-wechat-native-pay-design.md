# WeChat Native Pay Design

## Overview

Add WeChat Pay Native (扫码支付) as an independent payment channel, directly integrating with WeChat Pay API v3 using the official `wechatpay-go` SDK. Follows the same architectural pattern as existing Stripe/Waffo channels.

## Architecture

Independent payment channel with dedicated config, controller, routes, and frontend — consistent with Stripe/Waffo pattern.

## Backend

### Configuration (`setting/payment_wechat_pay.go`)

Variables registered via `common.OptionMap`:

| Variable | Type | Description |
|---|---|---|
| `WeChatPayAppID` | string | WeChat Pay AppID |
| `WeChatPayMchID` | string | Merchant ID |
| `WeChatPayAPIv3Key` | string | API v3 key (sensitive, no frontend echo) |
| `WeChatPayPrivateKey` | string | Merchant RSA private key PEM (sensitive, no frontend echo) |
| `WeChatPaySerialNo` | string | Merchant certificate serial number |
| `WeChatPayUnitPrice` | float64 | Price per USD (CNY), default 7.0 |
| `WeChatPayMinTopUp` | int | Minimum top-up amount (CNY), default 1 |

Enable check: `isWeChatPayTopUpEnabled()` returns true when AppID, MchID, and APIv3Key are all non-empty.

### Payment Method Constant

Add to `model/topup.go`:

```go
PaymentMethodWeChatPay = "wechat_pay"
```

### Controller (`controller/topup_wechat_pay.go`)

#### `RequestWeChatPayPay` — `POST /api/user/wechat-pay/pay`

1. Validate amount >= min top-up
2. Calculate `payMoney` in CNY using `getPayMoney` logic
3. Generate `tradeNo`, create `TopUp` record with `PaymentMethod = "wechat_pay"`
4. Call WeChat Pay v3 Native API (`POST /v3/pay/transactions/native`)
5. Return `{ code_url: "weixin://wxpay/..." }`

#### `RequestWeChatPayAmount` — `POST /api/user/wechat-pay/amount`

Same pattern as Stripe amount endpoint — returns CNY amount for display.

#### `WeChatPayWebhook` — `POST /api/wechat-pay/webhook`

1. Verify HTTP signature using WeChat platform certificate
2. Decrypt notification body (AEAD_AES_256_GCM)
3. On `trade_state == SUCCESS`: call `model.RechargeWeChatPay`
4. Return `202 Accepted` (v3 spec)

#### `GetWeChatPayOrderStatus` — `GET /api/user/wechat-pay/status`

1. Require user auth, verify order belongs to current user
2. Return order status: `pending` / `success` / `expired`

### Recharge Logic (`model/topup.go`)

New `RechargeWeChatPay(tradeNo string, callerIp string) error` function:

- Transaction: lock order by trade_no → verify status pending → mark success → increase user quota
- Same pattern as `RechargeWaffo`
- Log outside transaction

### TopUp Info Integration (`controller/topup.go`)

In `GetTopUpInfo`:
- Add `enable_wechat_pay_topup` flag
- Add `wechat_pay_min_topup` value
- Append WeChat Pay to `payMethods` list when enabled

### Routes (`router/api-router.go`)

```
POST /api/wechat-pay/webhook                    → WeChatPayWebhook
POST /api/user/wechat-pay/pay                   → RequestWeChatPayPay (UserAuth + CriticalRateLimit)
POST /api/user/wechat-pay/amount                → RequestWeChatPayAmount (UserAuth)
GET  /api/user/wechat-pay/status?trade_no=xxx   → GetWeChatPayOrderStatus (UserAuth)
```

### Option Registration (`controller/option.go`)

Register new WeChat Pay variables so they can be read/written via `/api/option/`.

### Webhook Availability

Add WeChat Pay to the webhook availability guard (similar to existing Stripe/Creem/Waffo pattern).

## Frontend

### Admin Config Page

New file: `web/src/pages/Setting/Payment/SettingsPaymentGatewayWeChatPay.jsx`

- Form fields matching all backend config variables
- Sensitive fields (APIv3Key, PrivateKey) use `type='password'`, no echo on load
- Banner showing webhook callback URL: `{ServerAddress}/api/wechat-pay/webhook`
- Submit via `/api/option/` API (same as Stripe config page)

Integration: Add to `SettingsPaymentGateway.jsx` or the payment settings index.

### TopUp Page Changes

In `web/src/components/topup/index.jsx`:
- Add `enableWeChatPayTopUp` state
- Add `wechatPayMinTopUp` state
- Handle `wechat_pay` type in `preTopUp` — call `/api/user/wechat-pay/pay` instead of showing confirm modal
- On success, open WeChatPayQRCodeModal with the returned `code_url`

### QR Code Modal

New file: `web/src/components/topup/modals/WeChatPayQRCodeModal.jsx`

- Display QR code generated from `code_url` using `qrcode.react` library
- Poll `GET /api/user/wechat-pay/status?trade_no=xxx` every 3 seconds
- On status `success`: close modal, refresh user quota, show success toast
- On timeout (5 minutes): stop polling, show expired message
- Allow manual close

### RechargeCard Integration

In `web/src/components/topup/RechargeCard.jsx`:
- Add WeChat Pay button with `SiWechat` icon (green)
- Handle `wechat_pay` type in payment method rendering

### PaymentConfirmModal

Add `wechat_pay` icon handling alongside existing alipay/wxpay/stripe.

### Dependency

Install `qrcode.react` via bun in `web/` directory.

## Data Flow

```
User clicks "微信支付"
  → POST /api/user/wechat-pay/pay { amount }
  → Backend: create TopUp order + call WeChat Native API
  → Return code_url
  → Frontend: open QRCodeModal with code_url
  → User scans QR with WeChat app, pays
  → WeChat sends POST /api/wechat-pay/webhook
  → Backend: verify signature → decrypt → RechargeWeChatPay
  → Frontend: polls /api/user/wechat-pay/status → detects success
  → Close modal, refresh quota
```

## Security Considerations

- Webhook signature verification using WeChat platform certificate (v3 standard)
- Order ownership check in status polling endpoint
- Sensitive config fields not echoed to frontend
- Transaction-level locking on order processing (prevents double-charge)
- Same CSRF/auth middleware as other payment endpoints
