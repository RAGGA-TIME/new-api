package service

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// ============================================================
// 微信公众号带参数二维码扫码登录服务
// ============================================================

// ScanStatus 扫码状态
type ScanStatus string

const (
	ScanStatusWaiting  ScanStatus = "waiting"  // 等待扫码
	ScanStatusScanned  ScanStatus = "scanned"  // 已扫码，待确认
	ScanStatusConfirmed ScanStatus = "confirmed" // 已确认，可登录
	ScanStatusExpired  ScanStatus = "expired"  // 已过期
)

// ScanRecord 扫码记录
type ScanRecord struct {
	Status     ScanStatus `json:"status"`
	OpenID     string     `json:"openid"`
	SceneStr   string     `json:"scene_str"`
	CreatedAt  int64      `json:"created_at"`
	BindMode   bool       `json:"bind_mode"`   // 是否绑定模式
	UserID     int        `json:"user_id"`      // 绑定模式下的用户ID
}

// WeChatQRCodeResponse 创建二维码的微信API响应
type WeChatQRCodeResponse struct {
	Ticket        string `json:"ticket"`
	ExpireSeconds int    `json:"expire_seconds"`
	URL           string `json:"url"`
	ErrCode       int    `json:"errcode"`
	ErrMsg        string `json:"errmsg"`
}

// WeChatAccessTokenResponse access_token API响应
type WeChatAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

// WeChatEventMessage 微信事件推送消息
type WeChatEventMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Event        string   `xml:"Event"`
	EventKey     string   `xml:"EventKey"`
	Ticket       string   `xml:"Ticket"`
}

// scanStore 扫码状态存储
var (
	scanStore     = make(map[string]*ScanRecord)
	scanStoreLock sync.RWMutex
)

// accessTokenCache access_token缓存
var (
	accessTokenValue   string
	accessTokenExpireAt int64
	accessTokenLock    sync.Mutex
)

// QRCodeExpireSeconds 二维码有效期（秒）
const QRCodeExpireSeconds = 300

// ScanRecordExpireSeconds 扫码记录有效期（秒）
const ScanRecordExpireSeconds = 360

// IsWeChatOffiAccountEnabled 判断是否启用了公众号带参数二维码模式
func IsWeChatOffiAccountEnabled() bool {
	return common.WeChatOffiAccountAppID != "" && common.WeChatOffiAccountAppSecret != ""
}

// ============================================================
// access_token 管理
// ============================================================

// GetWeChatAccessToken 获取微信公众号access_token（带缓存）
func GetWeChatAccessToken() (string, error) {
	accessTokenLock.Lock()
	defer accessTokenLock.Unlock()

	// 检查缓存是否有效（提前200秒刷新）
	if accessTokenValue != "" && time.Now().Unix() < accessTokenExpireAt-200 {
		return accessTokenValue, nil
	}

	// 请求新的access_token
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		common.WeChatOffiAccountAppID, common.WeChatOffiAccountAppSecret)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("请求微信access_token失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取access_token响应失败: %v", err)
	}

	var result WeChatAccessTokenResponse
	if err := parseJSON(body, &result); err != nil {
		return "", fmt.Errorf("解析access_token响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("获取access_token失败: [%d] %s", result.ErrCode, result.ErrMsg)
	}

	// 缓存access_token
	accessTokenValue = result.AccessToken
	accessTokenExpireAt = time.Now().Unix() + int64(result.ExpiresIn)

	common.SysLog(fmt.Sprintf("微信access_token已更新，有效期: %d秒", result.ExpiresIn))

	return accessTokenValue, nil
}

// ============================================================
// 二维码创建
// ============================================================

// CreateWeChatQRCode 创建带参数的临时二维码
func CreateWeChatQRCode(sceneStr string) (string, error) {
	accessToken, err := GetWeChatAccessToken()
	if err != nil {
		return "", err
	}

	// 创建临时二维码请求
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/qrcode/create?access_token=%s", accessToken)

	requestBody := fmt.Sprintf(`{"expire_seconds": %d, "action_name": "QR_STR_SCENE", "action_info": {"scene": {"scene_str": "%s"}}}`,
		QRCodeExpireSeconds, sceneStr)

	resp, err := http.Post(url, "application/json", strings.NewReader(requestBody))
	if err != nil {
		return "", fmt.Errorf("请求微信创建二维码失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取创建二维码响应失败: %v", err)
	}

	var result WeChatQRCodeResponse
	if err := parseJSON(body, &result); err != nil {
		return "", fmt.Errorf("解析创建二维码响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		// access_token过期时清除缓存重试
		if result.ErrCode == 40001 || result.ErrCode == 42001 {
			accessTokenLock.Lock()
			accessTokenValue = ""
			accessTokenExpireAt = 0
			accessTokenLock.Unlock()
		}
		return "", fmt.Errorf("创建二维码失败: [%d] %s", result.ErrCode, result.ErrMsg)
	}

	// 返回二维码图片URL
	qrcodeURL := fmt.Sprintf("https://mp.weixin.qq.com/cgi-bin/showqrcode?ticket=%s", encodeTicket(result.Ticket))
	return qrcodeURL, nil
}

// ============================================================
// 扫码状态管理
// ============================================================

// GenerateSceneStr 生成唯一的scene_str
func GenerateSceneStr() string {
	return fmt.Sprintf("login_%d_%s", time.Now().UnixMilli(), common.GetRandomString(8))
}

// SetScanRecord 保存扫码记录
func SetScanRecord(sceneStr string, record *ScanRecord) {
	scanStoreLock.Lock()
	defer scanStoreLock.Unlock()
	record.SceneStr = sceneStr
	record.CreatedAt = time.Now().Unix()
	scanStore[sceneStr] = record
}

// GetScanRecord 获取扫码记录
func GetScanRecord(sceneStr string) (*ScanRecord, bool) {
	scanStoreLock.RLock()
	defer scanStoreLock.RUnlock()
	record, ok := scanStore[sceneStr]
	if !ok {
		return nil, false
	}
	// 检查是否过期
	if time.Now().Unix()-record.CreatedAt > ScanRecordExpireSeconds {
		return nil, false
	}
	return record, true
}

// DeleteScanRecord 删除扫码记录
func DeleteScanRecord(sceneStr string) {
	scanStoreLock.Lock()
	defer scanStoreLock.Unlock()
	delete(scanStore, sceneStr)
}

// UpdateScanStatus 更新扫码状态
func UpdateScanStatus(sceneStr string, status ScanStatus, openID string) {
	scanStoreLock.Lock()
	defer scanStoreLock.Unlock()
	if record, ok := scanStore[sceneStr]; ok {
		record.Status = status
		if openID != "" {
			record.OpenID = openID
		}
	}
}

// CleanExpiredScanRecords 清理过期的扫码记录
func CleanExpiredScanRecords() {
	scanStoreLock.Lock()
	defer scanStoreLock.Unlock()
	now := time.Now().Unix()
	for key, record := range scanStore {
		if now-record.CreatedAt > ScanRecordExpireSeconds {
			delete(scanStore, key)
		}
	}
}

// StartScanRecordCleaner 启动定时清理任务
func StartScanRecordCleaner() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			CleanExpiredScanRecords()
		}
	}()
}

// ============================================================
// 微信回调验证
// ============================================================

// VerifyWeChatCallback 验证微信回调URL的signature
func VerifyWeChatCallback(signature, timestamp, nonce string) bool {
	token := common.WeChatOffiAccountToken
	if token == "" {
		return false
	}

	// 将token、timestamp、nonce三个参数进行字典序排序
	strs := []string{token, timestamp, nonce}
	sort.Strings(strs)

	// 将三个参数字符串拼接成一个字符串进行sha1加密
	h := sha1.New()
	for _, s := range strs {
		h.Write([]byte(s))
	}
	calculated := hex.EncodeToString(h.Sum(nil))

	return calculated == signature
}

// ParseWeChatCallback 解析微信事件推送
func ParseWeChatCallback(body []byte) (*WeChatEventMessage, error) {
	var msg WeChatEventMessage
	if err := xml.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("解析微信事件推送失败: %v", err)
	}
	return &msg, nil
}

// HandleWeChatEvent 处理微信事件
func HandleWeChatEvent(msg *WeChatEventMessage) {
	// 提取scene_str
	// 关注事件: EventKey = "qrscene_xxx"
	// 扫码事件(已关注): EventKey = "xxx"
	sceneStr, _ := strings.CutPrefix(msg.EventKey, "qrscene_")

	if sceneStr == "" {
		return
	}

	openID := msg.FromUserName

	// 检查扫码记录是否存在
	_, ok := GetScanRecord(sceneStr)
	if !ok {
		common.SysLog(fmt.Sprintf("微信事件推送: 未找到scene_str=%s的记录", sceneStr))
		return
	}

	// 更新状态
	switch msg.Event {
	case "subscribe", "SCAN":
		// 用户扫码关注或已关注用户扫码
		UpdateScanStatus(sceneStr, ScanStatusConfirmed, openID)
		common.SysLog(fmt.Sprintf("微信扫码成功: scene_str=%s, openid=%s, event=%s", sceneStr, openID, msg.Event))
	default:
		common.SysLog(fmt.Sprintf("微信事件推送: 未处理的事件类型=%s, scene_str=%s", msg.Event, sceneStr))
	}
}

// ============================================================
// 辅助函数
// ============================================================

// parseJSON 解析JSON
func parseJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// encodeTicket 对ticket进行URL编码
func encodeTicket(ticket string) string {
	return strings.ReplaceAll(ticket, "+", "%2B")
}
